package valkey

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setup(t *testing.T) (*ValkeyStore, *miniredis.Miniredis) {
	t.Helper()
	srv := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewWithClient(rdb, "so1:p2:"), srv
}

func TestPingAndSaveSystem(t *testing.T) {
	store, _ := setup(t)
	ctx := context.Background()

	if err := store.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	e := SnapshotEntry{TimestampUnixNS: time.Now().UnixNano(), MemTotalKB: 100, MemFreeKB: 40, MemUsedKB: 60, ProcessCount: 3}
	if err := store.SaveSystemLatest(ctx, e); err != nil {
		t.Fatalf("save system:latest: %v", err)
	}
	if err := store.AppendSystemTimeline(ctx, e); err != nil {
		t.Fatalf("append system:timeline: %v", err)
	}
}

func TestContainersTimelineAndRankings(t *testing.T) {
	store, srv := setup(t)
	ctx := context.Background()

	entries := []ContainerEntry{
		{ContainerID: "c1", Name: "alpha", HostPID: 1, MemPercent: 50, RSSKB: 500, CPU: 2.0},
		{ContainerID: "c2", Name: "beta", HostPID: 2, MemPercent: 20, RSSKB: 200, CPU: 5.0},
	}
	if err := store.AppendContainersTimeline(ctx, entries); err != nil {
		t.Fatalf("containers timeline: %v", err)
	}
	if err := store.AddRankings(ctx, entries); err != nil {
		t.Fatalf("add rankings: %v", err)
	}

	if got, err := srv.ZScore("so1:p2:rank:ram", "c1"); err != nil || got != 500 {
		t.Errorf("rank ram c1 = %v (%v), esperaba 500", got, err)
	}
	if got, err := srv.ZScore("so1:p2:rank:cpu", "c2"); err != nil || got != 5 {
		t.Errorf("rank cpu c2 = %v (%v), esperaba 5", got, err)
	}
}

func TestRecordKillAndCount(t *testing.T) {
	store, srv := setup(t)
	ctx := context.Background()

	ev := KillEvent{ContainerID: "c1", HostPID: 1, Signal: "SIGKILL", TimestampUnixNS: time.Now().UnixNano()}
	if err := store.RecordKill(ctx, ev); err != nil {
		t.Fatalf("record kill: %v", err)
	}
	if err := store.RecordKill(ctx, ev); err != nil {
		t.Fatalf("record kill 2: %v", err)
	}

	count, err := store.KillCount(ctx)
	if err != nil {
		t.Fatalf("kill count: %v", err)
	}
	if count != 2 {
		t.Errorf("kill count = %d, esperaba 2", count)
	}
	if got, err := srv.Stream("so1:p2:kills"); err != nil || len(got) != 2 {
		t.Errorf("stream kills = %d (err %v), esperaba 2", len(got), err)
	}
}

func TestKillCountEmpty(t *testing.T) {
	store, _ := setup(t)
	count, err := store.KillCount(context.Background())
	if err != nil {
		t.Fatalf("kill count vacío: %v", err)
	}
	if count != 0 {
		t.Errorf("kill count vacío = %d, esperaba 0", count)
	}
}

func TestRecordKillStreamFields(t *testing.T) {
	store, srv := setup(t)
	ctx := context.Background()

	ev := KillEvent{ContainerID: "c-abc", HostPID: 321, Signal: "SIGTERM", TimestampUnixNS: 42}
	if err := store.RecordKill(ctx, ev); err != nil {
		t.Fatalf("record kill: %v", err)
	}

	entries, err := srv.Stream("so1:p2:kills")
	if err != nil || len(entries) != 1 {
		t.Fatalf("stream kills = %d (err %v), esperaba 1", len(entries), err)
	}
	vals := map[string]string{}
	for i := 0; i+1 < len(entries[0].Values); i += 2 {
		vals[entries[0].Values[i]] = entries[0].Values[i+1]
	}
	if vals["container_id"] != "c-abc" {
		t.Errorf("container_id = %q, esperaba c-abc", vals["container_id"])
	}
	if vals["host_pid"] != "321" {
		t.Errorf("host_pid = %q, esperaba 321", vals["host_pid"])
	}
	if vals["signal"] != "SIGTERM" {
		t.Errorf("signal = %q, esperaba SIGTERM", vals["signal"])
	}
}

func TestAppendContainersEmpty(t *testing.T) {
	store, _ := setup(t)
	if err := store.AppendContainersTimeline(context.Background(), nil); err != nil {
		t.Fatalf("timeline vacío no debe fallar: %v", err)
	}
}
