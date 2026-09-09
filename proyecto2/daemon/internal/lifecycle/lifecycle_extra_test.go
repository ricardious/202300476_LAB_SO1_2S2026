package lifecycle

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"syscall"
	"testing"
	"time"

	"github.com/202300476/p2-daemon/internal/ebpf"
)

// failingConsumer implementa ebpf.Consumer y falla en Start con un error de
// privilegios (EPERM), simulando un host sin CAP_BPF/root.
type failingConsumer struct{}

func (failingConsumer) Events() <-chan ebpf.Event {
	ch := make(chan ebpf.Event)
	return ch
}
func (failingConsumer) Start(ctx context.Context) error {
	return &wrapErr{err: syscall.EPERM}
}
func (failingConsumer) Close() error { return nil }

// wrapErr envuelve un error para verificar errors.Is en el fallback.
type wrapErr struct{ err error }

func (w *wrapErr) Error() string { return "ebpf: carga: " + w.err.Error() }
func (w *wrapErr) Unwrap() error { return w.err }

func TestRunDegradesEBPFInAutoMode(t *testing.T) {
	d, _, _ := buildDaemon(t, failingConsumer{})
	d.Cfg.EBPFMode = "auto"
	d.Cfg.LoopInterval = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	if !d.Degraded() {
		t.Error("en modo auto con error de privilegios el daemon debe degradar a consumidor nulo")
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run con degradación debe terminar en nil: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run no terminó tras cancelar el contexto")
	}
}

func TestRunRequiredFailsWithoutPrivileges(t *testing.T) {
	d, _, _ := buildDaemon(t, failingConsumer{})
	d.Cfg.EBPFMode = "required"
	d.Cfg.LoopInterval = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.Run(ctx); !errors.Is(err, ErrConsumerFailed) {
		t.Fatalf("EBPFMode=required sin privilegios debe fallar con ErrConsumerFailed, obtuve: %v", err)
	}
}

func TestRunOffUsesNullConsumer(t *testing.T) {
	d, _, _ := buildDaemon(t, failingConsumer{})
	d.Cfg.EBPFMode = "off"
	d.Cfg.LoopInterval = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	if !d.Degraded() {
		t.Error("EBPFMode=off debe operar degradado (sin confirmación eBPF)")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run con modo off debe terminar en nil: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run no terminó")
	}
}

func TestModuleManagerLoadUnload(t *testing.T) {
	var calls []string
	exec := func(_ context.Context, path string) error {
		calls = append(calls, path)
		return nil
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := NewModuleManagerWithExecutor("load.sh", "unload.sh", logger, exec)

	if err := m.Load(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := m.Unload(context.Background()); err != nil {
		t.Fatalf("unload: %v", err)
	}
	if len(calls) != 2 || calls[0] != "load.sh" || calls[1] != "unload.sh" {
		t.Errorf("llamadas = %v, esperaba [load.sh unload.sh]", calls)
	}
}

func TestModuleManagerScriptVacioNoEjecuta(t *testing.T) {
	var calls int
	exec := func(_ context.Context, _ string) error { calls++; return nil }
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := NewModuleManagerWithExecutor("", "", logger, exec)

	if err := m.Load(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := m.Unload(context.Background()); err != nil {
		t.Fatalf("unload: %v", err)
	}
	if calls != 0 {
		t.Errorf("script vacío no debe ejecutarse (calls=%d)", calls)
	}
}

func TestModuleManagerPropagaError(t *testing.T) {
	exec := func(_ context.Context, _ string) error { return errors.New("insmod failed") }
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := NewModuleManagerWithExecutor("load.sh", "unload.sh", logger, exec)
	if err := m.Load(context.Background()); err == nil {
		t.Fatal("error del script debe propagarse")
	}
}

// fakePinger responde al ping según un flag.
type fakePinger struct{ ok bool }

func (f *fakePinger) Ping(ctx context.Context) error {
	if !f.ok {
		return errors.New("valkey: connection refused")
	}
	return nil
}

// fakeComposeRunner cuenta llamadas y puede "sanear" el pinger tras Up.
type fakeComposeRunner struct {
	ups, downs int
	recover    bool
	p          *fakePinger
}

func (f *fakeComposeRunner) Up(ctx context.Context, file string) error {
	f.ups++
	if f.recover {
		f.p.ok = true
	}
	return nil
}
func (f *fakeComposeRunner) Down(ctx context.Context, file string) error { f.downs++; return nil }

func TestComposeBootstrapLevantaStackSiValkeyCae(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	p := &fakePinger{ok: false}
	r := &fakeComposeRunner{recover: true, p: p}

	err := ComposeBootstrap(context.Background(), "deploy/docker-compose.yml", r, p, logger, 5*time.Second)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if r.ups != 1 {
		t.Errorf("compose up llamadas = %d, esperaba 1", r.ups)
	}
}

func TestComposeBootstrapFallaSiValkeySigueCayendo(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	p := &fakePinger{ok: false}
	r := &fakeComposeRunner{p: p} // no recupera

	err := ComposeBootstrap(context.Background(), "deploy/docker-compose.yml", r, p, logger, 2*time.Second)
	if err == nil {
		t.Fatal("si Valkey nunca responde, bootstrap debe fallar con mensaje accionable")
	}
	if r.ups != 1 {
		t.Errorf("compose up llamadas = %d, esperaba 1", r.ups)
	}
}

func TestComposeBootstrapNoHaceNadaSiValkeyResponde(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	p := &fakePinger{ok: true}
	r := &fakeComposeRunner{}

	err := ComposeBootstrap(context.Background(), "deploy/docker-compose.yml", r, p, logger, 5*time.Second)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if r.ups != 0 {
		t.Errorf("compose up llamadas = %d, esperaba 0 (Valkey ya responde)", r.ups)
	}
}

func TestComposeBootstrapSinArchivoEsNoOp(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	p := &fakePinger{ok: false}
	r := &fakeComposeRunner{}

	if err := ComposeBootstrap(context.Background(), "", r, p, logger, time.Second); err != nil {
		t.Fatalf("sin compose file debe ser no-op: %v", err)
	}
	if r.ups != 0 {
		t.Errorf("compose up llamadas = %d, esperaba 0", r.ups)
	}
}
