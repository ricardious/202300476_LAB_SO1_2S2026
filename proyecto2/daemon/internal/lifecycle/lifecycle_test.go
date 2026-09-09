package lifecycle

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/202300476/p2-daemon/internal/config"
	"github.com/202300476/p2-daemon/internal/docker"
	"github.com/202300476/p2-daemon/internal/ebpf"
	"github.com/202300476/p2-daemon/internal/policy"
	"github.com/202300476/p2-daemon/internal/valkey"
)

// fakeProc devuelve bytes fijos al leer la instantánea.
type fakeProc struct{ data []byte }

func (f fakeProc) Read() ([]byte, error) { return f.data, nil }

// fakeStore registra llamadas de persistencia.
type fakeStore struct {
	mu          sync.Mutex
	kills       []valkey.KillEvent
	systemSaved bool
}

func (s *fakeStore) Ping(ctx context.Context) error { return nil }
func (s *fakeStore) SaveSystemLatest(ctx context.Context, e valkey.SnapshotEntry) error {
	s.mu.Lock()
	s.systemSaved = true
	s.mu.Unlock()
	return nil
}
func (s *fakeStore) AppendSystemTimeline(ctx context.Context, e valkey.SnapshotEntry) error {
	return nil
}
func (s *fakeStore) AppendContainersTimeline(ctx context.Context, es []valkey.ContainerEntry) error {
	return nil
}
func (s *fakeStore) AddRankings(ctx context.Context, es []valkey.ContainerEntry) error { return nil }
func (s *fakeStore) RecordKill(ctx context.Context, ev valkey.KillEvent) error {
	s.mu.Lock()
	s.kills = append(s.kills, ev)
	s.mu.Unlock()
	return nil
}
func (s *fakeStore) KillCount(ctx context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return int64(len(s.kills)), nil
}

func (s *fakeStore) killCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.kills)
}

// fakeDocker implementa ContainerAPI con contenedores fijos.
type fakeDocker struct {
	managed []docker.Container
	byID    map[string]docker.Container
	killed  []string
}

func (d *fakeDocker) Ping(ctx context.Context) error { return nil }
func (d *fakeDocker) ListManaged(ctx context.Context, k, v string) ([]docker.Container, error) {
	return d.managed, nil
}
func (d *fakeDocker) Inspect(ctx context.Context, id string) (docker.Container, error) {
	c, ok := d.byID[id]
	if !ok {
		return docker.Container{}, errors.New("no existe")
	}
	return c, nil
}
func (d *fakeDocker) Kill(ctx context.Context, id, signal string) error {
	d.killed = append(d.killed, id)
	return nil
}

// procSnapshotJSON devuelve una snapshot con PIDs de contenedor.
func procSnapshotJSON() []byte {
	return []byte(`{
		"schema_version": 1,
		"timestamp_unix_ns": 1757100000000000000,
		"memory": {"total_kb": 1000, "free_kb": 500, "used_kb": 500},
		"processes": [
			{"pid": 100, "name": "c1", "container_id": "c1", "vsz_kb": 90000, "rss_kb": 9000, "memory_percent": 90, "cpu_percent": 1},
			{"pid": 200, "name": "c2", "container_id": "c2", "vsz_kb": 80000, "rss_kb": 8000, "memory_percent": 80, "cpu_percent": 1},
			{"pid": 300, "name": "c3", "container_id": "c3", "vsz_kb": 70000, "rss_kb": 7000, "memory_percent": 70, "cpu_percent": 1},
			{"pid": 400, "name": "c4", "container_id": "c4", "vsz_kb": 60000, "rss_kb": 6000, "memory_percent": 60, "cpu_percent": 1},
			{"pid": 500, "name": "c5", "container_id": "c5", "vsz_kb": 50000, "rss_kb": 5000, "memory_percent": 50, "cpu_percent": 1},
			{"pid": 600, "name": "c6", "container_id": "c6", "vsz_kb": 40000, "rss_kb": 4000, "memory_percent": 40, "cpu_percent": 1},
			{"pid": 700, "name": "c7", "container_id": "c7", "vsz_kb": 30000, "rss_kb": 3000, "memory_percent": 30, "cpu_percent": 1},
			{"pid": 800, "name": "c8", "container_id": "c8", "vsz_kb": 20000, "rss_kb": 2000, "memory_percent": 20, "cpu_percent": 1},
			{"pid": 900, "name": "c9", "container_id": "c9", "vsz_kb": 10000, "rss_kb": 1000, "memory_percent": 10, "cpu_percent": 1}
		]
	}`)
}

func buildDaemon(t *testing.T, cons ebpf.Consumer) (*Daemon, *fakeDocker, *fakeStore) {
	t.Helper()
	cfg := config.Default()
	cfg.ConfirmationTimeout = 5 * time.Second

	var managed []docker.Container
	byID := map[string]docker.Container{}
	pids := []int{100, 200, 300, 400, 500, 600, 700, 800, 900}
	for i, pid := range pids {
		id := "c" + string(rune('1'+i))
		c := docker.Container{ID: id, Name: "/" + id, HostPID: pid}
		managed = append(managed, c)
		byID[id] = c
	}
	fd := &fakeDocker{managed: managed, byID: byID}

	fs := &fakeStore{}
	corr := ebpf.NewCorrelator(5 * time.Second)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(cfg, fakeProc{data: procSnapshotJSON()}, fd, policy.New(cfg), fs, cons, corr, logger)
	d.ProcessKill = func(pid int, _ syscall.Signal) error {
		for id, c := range fd.byID {
			if c.HostPID == pid {
				fd.killed = append(fd.killed, id)
				return nil
			}
		}
		return errors.New("pid no existe")
	}
	return d, fd, fs
}

func TestReconcileRequestsKillsAndPreservesQuota(t *testing.T) {
	d, fd, _ := buildDaemon(t, ebpf.NewNullConsumer())

	res, err := d.reconcile(context.Background())
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	// 9 contenedores, mínimos 3+2 => 5 sobreviven, 4 candidatos.
	if res.SurvivorCount != 5 {
		t.Errorf("supervivientes = %d, esperaba 5", res.SurvivorCount)
	}
	if res.RemovalCandidates != 4 {
		t.Errorf("candidatos = %d, esperaba 4", res.RemovalCandidates)
	}
	if len(fd.killed) != 4 {
		t.Errorf("kills solicitados = %d, esperaba 4", len(fd.killed))
	}
	// Los de mayor consumo (c1,c2) nunca deben morir.
	for _, k := range fd.killed {
		if k == "c1" || k == "c2" {
			t.Errorf("no debe eliminarse %s (alto consumo)", k)
		}
	}
}

func TestKillOnlyConfirmedAfterEBPFEvent(t *testing.T) {
	cons := ebpf.NewMemConsumer(4)
	d, fd, fs := buildDaemon(t, cons)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Consumidor eBPF en background.
	go d.runEventConsumer(ctx)

	// Primer ciclo: solicita 4 kills y registra pendientes en el correlator.
	if _, err := d.reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if fs.killCount() != 0 {
		t.Fatalf("sin evento eBPF no debe confirmarse ninguna eliminación, count=%d", fs.killCount())
	}

	// Los candidatos de este escenario son c3..c6 (PIDs 300..600). El primero
	// en eliminarse (mayor consumo) es c3 con PID de host 300.
	const targetPID = 300
	if fd.killed[0] != "c3" {
		t.Fatalf("primer kill solicitado = %s, esperaba c3", fd.killed[0])
	}

	if err := cons.Emit(ebpf.Event{TargetPID: targetPID, Signal: 9}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	// Esperar a que el consumidor correlacione y registre el kill.
	deadline := time.After(2 * time.Second)
	for {
		if fs.killCount() == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("el kill correlacionado no se registró; pendientes=%d", d.Corr.PendingCount())
		case <-time.After(10 * time.Millisecond):
		}
	}

	// El kill registrado debe corresponder al contenedor confirmado.
	fs.mu.Lock()
	ev := fs.kills[0]
	fs.mu.Unlock()
	if ev.HostPID != targetPID {
		t.Errorf("kill confirmado host_pid = %d, esperaba %d", ev.HostPID, targetPID)
	}
	if ev.Signal != "SIGKILL" {
		t.Errorf("señal = %q, esperaba SIGKILL", ev.Signal)
	}
}

func TestRunStopsOnSignalContext(t *testing.T) {
	cons := ebpf.NewNullConsumer()
	d, _, _ := buildDaemon(t, cons)
	d.Cfg.LoopInterval = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	cancel() // equivale a recibir SIGINT/SIGTERM

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run debe terminar con nil en apagado ordenado: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run no terminó tras cancelar el contexto")
	}
}

func TestSignalNumber(t *testing.T) {
	if signalNumber("SIGKILL") != 9 {
		t.Error("SIGKILL -> 9")
	}
	if signalNumber("SIGTERM") != 15 {
		t.Error("SIGTERM -> 15")
	}
	if signalNumber("desconocida") != 9 {
		t.Error("señal desconocida debe caer a SIGKILL(9)")
	}
}
