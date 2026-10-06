package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"so1/proyecto3/internal/report"
	"so1/proyecto3/internal/store"
	pb "so1/proyecto3/proto"
	"strings"
	"sync"
	"syscall"
	"time"
)

func env(k, d string) string {
	if s := os.Getenv(k); s != "" {
		return s
	}
	return d
}

var client = &http.Client{Timeout: 4 * time.Second, Transport: &http.Transport{MaxIdleConns: 16, MaxIdleConnsPerHost: 8, IdleConnTimeout: 30 * time.Second}}

func forward(ctx context.Context, url string, r report.Report) error {
	b, _ := json.Marshal(r)
	req, e := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	resp, e := client.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("upstream returned %d", resp.StatusCode)
	}
	return nil
}
func httpServer(ctx context.Context, addr string, m *http.ServeMux) {
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, "ok") })
	s := &http.Server{Addr: addr, Handler: http.MaxBytesHandler(m, 4096), ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 8 * time.Second, IdleTimeout: 30 * time.Second}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		s.Shutdown(c)
	}()
	log.Printf("role=%s listening=%s", os.Getenv("ROLE"), addr)
	if e := s.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
		log.Fatal(e)
	}
}
func handler(next func(context.Context, report.Report) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, e := report.Decode(r.Body)
		if e != nil {
			http.Error(w, "invalid report", 422)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if e = next(ctx, v); e != nil {
			log.Printf("report_id=%s error=%v", v.ID, e)
			http.Error(w, "upstream unavailable", 503)
			return
		}
		log.Printf("report_id=%s country=%s status=accepted", v.ID, v.Country)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		fmt.Fprintln(w, `{"status":"queued"}`)
	}
}
func ingest(ctx context.Context) {
	m := http.NewServeMux()
	m.HandleFunc("POST /report", handler(func(c context.Context, r report.Report) error {
		return forward(c, env("CLIENT_URL", "http://127.0.0.1:8081/report"), r)
	}))
	httpServer(ctx, ":8080", m)
}
func rpcClient(ctx context.Context) {
	conn, e := grpc.NewClient(env("GRPC_ADDR", "go-writer:50051"), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultServiceConfig(`{"loadBalancingConfig":[{"round_robin":{}}]}`))
	if e != nil {
		log.Fatal(e)
	}
	defer conn.Close()
	c := pb.NewWarReportServiceClient(conn)
	m := http.NewServeMux()
	m.HandleFunc("POST /report", handler(func(ctx context.Context, r report.Report) error {
		country := pb.Countries(pb.Countries_value[strings.ToLower(r.Country)])
		_, e := c.SendReport(ctx, &pb.WarReportRequest{Country: country, WarplanesInAir: r.Planes, WarshipsInWater: r.Ships, Timestamp: r.Timestamp, ReportId: r.ID})
		return e
	}))
	httpServer(ctx, ":8081", m)
}

type rpcServer struct {
	pb.UnimplementedWarReportServiceServer
}

func (s *rpcServer) SendReport(ctx context.Context, p *pb.WarReportRequest) (*pb.WarReportResponse, error) {
	r := report.Report{Country: strings.ToUpper(p.Country.String()), Planes: p.WarplanesInAir, Ships: p.WarshipsInWater, Timestamp: p.Timestamp, ID: p.ReportId}
	if e := r.Validate(); e != nil {
		return nil, status.Error(codes.InvalidArgument, e.Error())
	}
	if e := forward(ctx, env("WRITER_URL", "http://127.0.0.1:8082/report"), r); e != nil {
		return nil, status.Error(codes.Unavailable, "writer unavailable")
	}
	return &pb.WarReportResponse{Status: "queued"}, nil
}
func serveGRPC(ctx context.Context) {
	l, e := net.Listen("tcp", ":50051")
	if e != nil {
		log.Fatal(e)
	}
	s := grpc.NewServer(grpc.MaxRecvMsgSize(4096))
	pb.RegisterWarReportServiceServer(s, &rpcServer{})
	go func() { <-ctx.Done(); s.Stop() }()
	log.Println("gRPC listening :50051")
	if e = s.Serve(l); e != nil && ctx.Err() == nil {
		log.Fatal(e)
	}
}
func queue(ch *amqp.Channel) error {
	_, e := ch.QueueDeclare("reports", true, false, false, false, amqp.Table{"x-message-ttl": int64(86400000), "x-max-length": int64(10000), "x-overflow": "reject-publish", "x-dead-letter-exchange": "reports.dlx"})
	return e
}
func connect() (*amqp.Connection, *amqp.Channel, error) {
	conn, e := amqp.DialConfig(env("AMQP_URL", "amqp://guest:guest@rabbitmq:5672/"), amqp.Config{Heartbeat: 10 * time.Second, Dial: amqp.DefaultDial(3 * time.Second)})
	if e != nil {
		return nil, nil, e
	}
	ch, e := conn.Channel()
	if e != nil {
		conn.Close()
		return nil, nil, e
	}
	if e = ch.ExchangeDeclare("reports.dlx", "fanout", true, false, false, false, nil); e == nil {
		_, e = ch.QueueDeclare("reports.dead", true, false, false, false, amqp.Table{"x-max-length": int64(1000)})
	}
	if e == nil {
		e = ch.QueueBind("reports.dead", "", "reports.dlx", false, nil)
	}
	if e == nil {
		e = queue(ch)
	}
	if e != nil {
		ch.Close()
		conn.Close()
		return nil, nil, e
	}
	return conn, ch, nil
}

type publisher struct {
	mu      sync.Mutex
	conn    *amqp.Connection
	ch      *amqp.Channel
	returns <-chan amqp.Return
}

func (p *publisher) publish(ctx context.Context, r report.Report) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn == nil || p.conn.IsClosed() || p.ch.IsClosed() {
		if p.conn != nil {
			p.conn.Close()
		}
		var e error
		p.conn, p.ch, e = connect()
		if e != nil {
			return e
		}
		if e = p.ch.Confirm(false); e != nil {
			p.conn.Close()
			return e
		}
		p.returns = p.ch.NotifyReturn(make(chan amqp.Return, 1))
	}
	b, _ := json.Marshal(r)
	confirmation, e := p.ch.PublishWithDeferredConfirmWithContext(ctx, "", "reports", true, false, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, MessageId: r.ID, Timestamp: time.Now(), Body: b})
	if e != nil {
		p.conn.Close()
		return e
	}
	ok, e := confirmation.WaitContext(ctx)
	if e != nil || !ok {
		p.conn.Close()
		return fmt.Errorf("broker confirmation failed: %v", e)
	}
	select {
	case returned := <-p.returns:
		return fmt.Errorf("unroutable publication: %s", returned.ReplyText)
	default:
	}
	return nil
}
func writer(ctx context.Context) {
	p := &publisher{}
	defer func() {
		if p.conn != nil {
			p.conn.Close()
		}
	}()
	m := http.NewServeMux()
	m.HandleFunc("POST /report", handler(p.publish))
	httpServer(ctx, ":8082", m)
}
func consumeSession(ctx context.Context, c *redis.Client) error {
	conn, ch, e := connect()
	if e != nil {
		return e
	}
	defer conn.Close()
	defer ch.Close()
	if e = ch.Qos(16, 0, false); e != nil {
		return e
	}
	msgs, e := ch.Consume("reports", "", false, false, false, false, nil)
	if e != nil {
		return e
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-msgs:
			if !ok {
				return errors.New("broker disconnected")
			}
			r, e := report.Decode(bytes.NewReader(msg.Body))
			if e != nil {
				msg.Nack(false, false)
				continue
			}
			call, cancel := context.WithTimeout(ctx, 2*time.Second)
			applied, e := store.Apply(call, c, r)
			cancel()
			if e != nil {
				msg.Nack(false, true)
				log.Printf("storage error: %v", e)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Second):
				}
				continue
			}
			if e = msg.Ack(false); e != nil {
				return e
			}
			log.Printf("report_id=%s stored=%t", r.ID, applied)
		}
	}
}
func consumer(ctx context.Context) {
	c := redis.NewClient(&redis.Options{Addr: env("VALKEY_ADDR", "valkey:6379"), Password: os.Getenv("VALKEY_PASSWORD"), PoolSize: 4, DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second})
	defer c.Close()
	go func() {
		for ctx.Err() == nil {
			if e := consumeSession(ctx, c); e != nil && ctx.Err() == nil {
				log.Printf("consumer reconnect: %v", e)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}()
	m := http.NewServeMux()
	m.HandleFunc("GET /metrics", store.Metrics(c))
	httpServer(ctx, ":9091", m)
}
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	switch env("ROLE", "") {
	case "api":
		ingest(ctx)
	case "client":
		rpcClient(ctx)
	case "server":
		serveGRPC(ctx)
	case "writer":
		writer(ctx)
	case "consumer":
		consumer(ctx)
	default:
		log.Fatal("ROLE must be api/client/server/writer/consumer")
	}
}
