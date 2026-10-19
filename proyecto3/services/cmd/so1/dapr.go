package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	dapr "github.com/dapr/go-sdk/client"
	"github.com/dapr/go-sdk/service/common"
	daprhttp "github.com/dapr/go-sdk/service/http"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"so1/proyecto3/internal/report"
	"so1/proyecto3/internal/store"
)

const daprPubsub = "military-pubsub"
const daprTopic = "military-reports-dapr"

// Returning an error with retry=true makes Dapr retry a transient storage failure.
// Invalid messages are dropped; duplicates succeed after the shared atomic Lua script.
func daprEventHandler(apply func(context.Context, report.Report) (bool, error)) common.TopicEventHandler {
	return func(ctx context.Context, event *common.TopicEvent) (bool, error) {
		r, err := report.Decode(bytes.NewReader(event.RawData))
		if err != nil {
			log.Printf("flow=dapr invalid_event=%s", event.ID)
			return false, fmt.Errorf("invalid report: %w", err)
		}
		call, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		applied, err := apply(call, r)
		if err != nil {
			log.Printf("flow=dapr report_id=%s storage_error=%v", r.ID, err)
			select {
			case <-ctx.Done():
			case <-time.After(500 * time.Millisecond):
			}
			return true, err
		}
		log.Printf("flow=dapr report_id=%s stored=%t", r.ID, applied)
		return false, nil
	}
}

func daprFlow(ctx context.Context) {
	c := redis.NewClient(&redis.Options{Addr: env("VALKEY_ADDR", "valkey:6379"), Password: env("VALKEY_PASSWORD", ""), PoolSize: 4, DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second})
	defer c.Close()
	// Non-blocking connection: the subscription callback must start before daprd
	// becomes fully healthy. Waiting for readiness here would deadlock startup.
	conn, err := grpc.NewClient("127.0.0.1:50001", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	sdk := dapr.NewClientWithConnection(conn)
	defer sdk.Close()
	mux := chi.NewRouter()
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, 8192) // Includes the CloudEvent envelope.
			call, cancel := context.WithTimeout(r.Context(), 4*time.Second)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(call))
		})
	})
	subscriber := daprhttp.NewServiceWithMux("127.0.0.1:8084", mux)
	err = subscriber.AddTopicEventHandler(&common.Subscription{PubsubName: daprPubsub, Topic: daprTopic, Route: "/events/reports"}, daprEventHandler(func(ctx context.Context, r report.Report) (bool, error) {
		return store.Apply(ctx, c, r)
	}))
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		if err := subscriber.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	go func() { <-ctx.Done(); _ = subscriber.Stop() }()
	m := http.NewServeMux()
	m.HandleFunc("POST /report", handler(func(ctx context.Context, r report.Report) error {
		return sdk.PublishEvent(ctx, daprPubsub, daprTopic, r)
	}))
	m.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		call, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(call, http.MethodGet, "http://127.0.0.1:3500/v1.0/healthz", nil)
		res, err := client.Do(req)
		if err != nil {
			http.Error(w, "sidecar unavailable", 503)
			return
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusNoContent {
			http.Error(w, "sidecar not ready", 503)
			return
		}
		fmt.Fprintln(w, "ok")
	})
	httpServer(ctx, ":8083", m)
}
