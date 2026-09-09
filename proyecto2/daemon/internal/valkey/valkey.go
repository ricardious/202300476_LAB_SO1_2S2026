// Package valkey persiste la telemetría y los eventos del proyecto en Valkey.
//
// Usa el prefijo de claves so1:p2: con los siguientes elementos:
//   - system:latest        hash de la instantánea más reciente
//   - system:timeline      stream temporal de RAM
//   - containers:timeline  stream histórico de contenedores
//   - rank:ram / rank:cpu  sorted sets de rankings históricos
//   - kills                stream de eliminaciones confirmadas por eBPF
//   - kills:count          contador total confirmado
//
// La abstracción Store permite inyectar un back-end en memoria en las pruebas.
package valkey

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// SnapshotEntry es la vista persistible de una instantánea del sistema.
type SnapshotEntry struct {
	TimestampUnixNS int64
	MemTotalKB      int64
	MemFreeKB       int64
	MemUsedKB       int64
	ProcessCount    int
}

// ContainerEntry es la métrica histórica de un contenedor.
type ContainerEntry struct {
	ContainerID     string
	Name            string
	HostPID         int
	MemPercent      float64
	VSZKB           int64
	RSSKB           int64
	CPU             float64
	TimestampUnixNS int64
}

// KillEvent es una eliminación confirmada por el evento eBPF correlacionado.
type KillEvent struct {
	ContainerID     string
	HostPID         int
	Signal          string
	TimestampUnixNS int64
}

// Store es el contrato de persistencia que consume el daemon.
type Store interface {
	Ping(ctx context.Context) error
	SaveSystemLatest(ctx context.Context, e SnapshotEntry) error
	AppendSystemTimeline(ctx context.Context, e SnapshotEntry) error
	AppendContainersTimeline(ctx context.Context, entries []ContainerEntry) error
	AddRankings(ctx context.Context, entries []ContainerEntry) error
	RecordKill(ctx context.Context, ev KillEvent) error
	KillCount(ctx context.Context) (int64, error)
}

// ValkeyStore implementa Store sobre go-redis (compatible con Valkey).
type ValkeyStore struct {
	rdb    *redis.Client
	prefix string
}

// New construye un ValkeyStore con la dirección y prefijo dados.
func New(addr, password string, db int, prefix string) *ValkeyStore {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &ValkeyStore{rdb: rdb, prefix: prefix}
}

// NewWithClient construye un ValkeyStore a partir de un cliente ya creado.
func NewWithClient(rdb *redis.Client, prefix string) *ValkeyStore {
	return &ValkeyStore{rdb: rdb, prefix: prefix}
}

func (s *ValkeyStore) key(suffix string) string { return s.prefix + suffix }

// Ping implementa Store.Ping.
func (s *ValkeyStore) Ping(ctx context.Context) error {
	if err := s.rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("valkey: ping: %w", err)
	}
	return nil
}

// SaveSystemLatest implementa Store.SaveSystemLatest.
func (s *ValkeyStore) SaveSystemLatest(ctx context.Context, e SnapshotEntry) error {
	if err := s.rdb.HSet(ctx, s.key("system:latest"), map[string]interface{}{
		"timestamp_unix_ns": e.TimestampUnixNS,
		"mem_total_kb":      e.MemTotalKB,
		"mem_free_kb":       e.MemFreeKB,
		"mem_used_kb":       e.MemUsedKB,
		"process_count":     e.ProcessCount,
	}).Err(); err != nil {
		return fmt.Errorf("valkey: save system:latest: %w", err)
	}
	return nil
}

// AppendSystemTimeline implementa Store.AppendSystemTimeline.
func (s *ValkeyStore) AppendSystemTimeline(ctx context.Context, e SnapshotEntry) error {
	fields := map[string]interface{}{
		"ts":            e.TimestampUnixNS,
		"mem_total_kb":  e.MemTotalKB,
		"mem_free_kb":   e.MemFreeKB,
		"mem_used_kb":   e.MemUsedKB,
		"process_count": e.ProcessCount,
	}
	if _, err := s.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: s.key("system:timeline"),
		Values: fields,
	}).Result(); err != nil {
		return fmt.Errorf("valkey: append system:timeline: %w", err)
	}
	return nil
}

// AppendContainersTimeline implementa Store.AppendContainersTimeline.
func (s *ValkeyStore) AppendContainersTimeline(ctx context.Context, entries []ContainerEntry) error {
	if len(entries) == 0 {
		return nil
	}
	pipe := s.rdb.Pipeline()
	for _, e := range entries {
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: s.key("containers:timeline"),
			Values: map[string]interface{}{
				"container_id": e.ContainerID,
				"name":         e.Name,
				"host_pid":     e.HostPID,
				"mem_percent":  e.MemPercent,
				"vsz_kb":       e.VSZKB,
				"rss_kb":       e.RSSKB,
				"cpu":          e.CPU,
				"ts":           e.TimestampUnixNS,
			},
		})
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("valkey: append containers:timeline: %w", err)
	}
	return nil
}

// AddRankings implementa Store.AddRankings.
//
// El miembro del sorted set codifica el ID del contenedor y su PID de host
// ("<id>|pid=<pid>") para que los paneles "Top 5 histórico por RAM/CPU"
// muestren PID e ID, como exige el dashboard de la rúbrica.
func (s *ValkeyStore) AddRankings(ctx context.Context, entries []ContainerEntry) error {
	if len(entries) == 0 {
		return nil
	}
	pipe := s.rdb.Pipeline()
	for _, e := range entries {
		member := rankMember(e.ContainerID, e.HostPID)
		pipe.ZAdd(ctx, s.key("rank:ram"), redis.Z{Score: float64(e.RSSKB), Member: member})
		pipe.ZAdd(ctx, s.key("rank:cpu"), redis.Z{Score: e.CPU, Member: member})
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("valkey: add rankings: %w", err)
	}
	return nil
}

// rankMember formatea el miembro de un sorted set con ID y PID.
func rankMember(containerID string, hostPID int) string {
	if hostPID > 0 {
		return fmt.Sprintf("%s|pid=%d", containerID, hostPID)
	}
	return containerID
}

// RecordKill implementa Store.RecordKill: registra la eliminación confirmada y
// actualiza el contador total.
func (s *ValkeyStore) RecordKill(ctx context.Context, ev KillEvent) error {
	pipe := s.rdb.Pipeline()
	pipe.XAdd(ctx, &redis.XAddArgs{
		Stream: s.key("kills"),
		Values: map[string]interface{}{
			"container_id": ev.ContainerID,
			"host_pid":     ev.HostPID,
			"signal":       ev.Signal,
			"ts":           ev.TimestampUnixNS,
		},
	})
	pipe.Incr(ctx, s.key("kills:count"))
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("valkey: record kill: %w", err)
	}
	return nil
}

// KillCount implementa Store.KillCount.
func (s *ValkeyStore) KillCount(ctx context.Context) (int64, error) {
	n, err := s.rdb.Get(ctx, s.key("kills:count")).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("valkey: kill count: %w", err)
	}
	return n, nil
}
