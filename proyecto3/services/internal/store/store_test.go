package store

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"os"
	"reflect"
	"so1/proyecto3/internal/report"
	"testing"
)

func TestModeTie(t *testing.T) {
	n, ties := mode(map[string]string{"20": "2", "0": "2", "3": "1"})
	if n != 0 || !reflect.DeepEqual(ties, []string{"0", "20"}) {
		t.Fatalf("mode=%d ties=%v", n, ties)
	}
}
func TestValkeyAtomicAggregation(t *testing.T) {
	addr := os.Getenv("SO1_TEST_VALKEY")
	if addr == "" {
		t.Skip("requires project Valkey; uses dedicated DB 15")
	}
	c := redis.NewClient(&redis.Options{Addr: addr, DB: 15})
	defer c.Close()
	ctx := context.Background()
	if e := c.FlushDB(ctx).Err(); e != nil {
		t.Fatal(e)
	}
	defer c.FlushDB(ctx)
	samples := []report.Report{{Country: "ESP", Planes: 0, Ships: 30, Timestamp: "2026-10-19T12:00:01Z", ID: "test-a"}, {Country: "USA", Planes: 50, Ships: 0, Timestamp: "2026-10-19T12:00:00Z", ID: "test-b"}, {Country: "ESP", Planes: 0, Ships: 30, Timestamp: "2026-10-19T12:00:00Z", ID: "test-c"}}
	for _, r := range samples {
		ok, e := Apply(ctx, c, r)
		if e != nil || !ok {
			t.Fatalf("apply: %v %v", ok, e)
		}
	}
	if ok, e := Apply(ctx, c, samples[0]); e != nil || ok {
		t.Fatalf("duplicate: %v %v", ok, e)
	}
	h, e := c.HGetAll(ctx, Prefix+"stats").Result()
	if e != nil {
		t.Fatal(e)
	}
	for k, want := range map[string]string{"total": "3", "country:ESP": "2", "planes_min": "0", "planes_max": "50", "ships_min": "0", "ships_max": "30"} {
		if h[k] != want {
			t.Fatal(fmt.Sprintf("%s=%s want %s", k, h[k], want))
		}
	}
	if v := c.HGet(ctx, Prefix+"latest:ESP", "timestamp").Val(); v != "2026-10-19T12:00:01Z" {
		t.Fatal("out-of-order event overwrote latest", v)
	}
	if v := c.ZScore(ctx, Prefix+"ships:rank", "ESP").Val(); v != 60 {
		t.Fatal("ranking", v)
	}
}
