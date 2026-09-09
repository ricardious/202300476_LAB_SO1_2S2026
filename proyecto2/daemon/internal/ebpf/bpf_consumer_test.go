package ebpf

import (
	"context"
	"encoding/binary"
	"errors"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/cilium/ebpf"
	"golang.org/x/sys/unix"
)

// isPrivilegeError detecta errores de permisos al intentar cargar eBPF. En
// hosts sin CAP_BPF/root (o con unprivileged_bpf_disabled=2) la carga falla
// con EPERM/EACCES y las pruebas de integración se omiten en vez de fallar.
func isPrivilegeError(err error) bool {
	return errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES)
}

// TestNewBPFConsumerParsesSpec verifica que el objeto CO-RE embebido se
// parsea y expone los programas y el mapa esperados. No requiere privilegios.
func TestNewBPFConsumerParsesSpec(t *testing.T) {
	c, err := NewBPFConsumer()
	if err != nil {
		t.Fatalf("NewBPFConsumer: %v", err)
	}
	defer func() { _ = c.Close() }()

	for _, name := range []string{"handle_kill", "handle_tgkill"} {
		prog := c.spec.Programs[name]
		if prog == nil {
			t.Fatalf("programa %q ausente en el objeto CO-RE", name)
		}
		if prog.Type != ebpf.TracePoint {
			t.Errorf("programa %s: tipo %v, esperaba TracePoint", name, prog.Type)
		}
	}
	m := c.spec.Maps["events"]
	if m == nil {
		t.Fatal("mapa 'events' ausente")
	}
	if m.Type != ebpf.RingBuf {
		t.Errorf("mapa events: tipo %v, esperaba RingBuf", m.Type)
	}
}

// TestDecodeRingEvent verifica el decodificado binario del evento emitido por
// el programa C (kill_event) contra el contrato Event.
func TestDecodeRingEvent(t *testing.T) {
	var raw [24]byte
	binary.LittleEndian.PutUint64(raw[0:8], 1234567890)
	binary.LittleEndian.PutUint32(raw[8:12], 777) // emitter_pid
	binary.LittleEndian.PutUint32(raw[12:16], 42) // target_pid
	binary.LittleEndian.PutUint32(raw[16:20], 9)  // signal
	binary.LittleEndian.PutUint32(raw[20:24], 0)  // reserved

	ev, err := decodeRingEvent(raw[:])
	if err != nil {
		t.Fatalf("decodeRingEvent: %v", err)
	}
	if ev.EmitterPID != 777 || ev.TargetPID != 42 || ev.Signal != 9 || ev.TimestampNS != 1234567890 {
		t.Errorf("evento decodificado = %+v", ev)
	}
}

// TestDecodeRingEventTargetPIDNegativo cubre kill(-pgid): el pid receptor
// viaja como int32 con signo.
func TestDecodeRingEventTargetPIDNegativo(t *testing.T) {
	var raw [24]byte
	neg := int32(-1000)
	binary.LittleEndian.PutUint32(raw[12:16], uint32(neg))
	ev, err := decodeRingEvent(raw[:])
	if err != nil {
		t.Fatalf("decodeRingEvent: %v", err)
	}
	if ev.TargetPID != -1000 {
		t.Errorf("target_pid = %d, esperaba -1000", ev.TargetPID)
	}
}

// TestDecodeRingEventTamañoIncorrecto verifica el rechazo de muestras con
// longitud distinta al layout del evento.
func TestDecodeRingEventTamañoIncorrecto(t *testing.T) {
	if _, err := decodeRingEvent([]byte{1, 2, 3}); err == nil {
		t.Fatal("muestra corta debe rechazarse")
	}
	if _, err := decodeRingEvent(make([]byte, ringEventSize+1)); err == nil {
		t.Fatal("muestra larga debe rechazarse")
	}
}

// TestBPFConsumerCloseAntesDeStart garantiza que cerrar un consumidor nunca
// iniciado cierra el canal sin panic y sin liberar nada dos veces.
func TestBPFConsumerCloseAntesDeStart(t *testing.T) {
	c, err := NewBPFConsumer()
	if err != nil {
		t.Fatalf("NewBPFConsumer: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, ok := <-c.Events(); ok {
		t.Error("el canal debe cerrarse tras Close")
	}
	// Doble Close es idempotente.
	if err := c.Close(); err != nil {
		t.Errorf("segundo Close: %v", err)
	}
}

// TestBPFConsumerStartDespuesDeClose verifica que Start rechaza un consumidor
// ya cerrado.
func TestBPFConsumerStartDespuesDeClose(t *testing.T) {
	c, err := NewBPFConsumer()
	if err != nil {
		t.Fatalf("NewBPFConsumer: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := c.Start(context.Background()); err == nil {
		t.Fatal("Start sobre un consumidor cerrado debe fallar")
	}
}

// TestBPFConsumerIntegracionPrivilegiada carga el consumidor real, adjunta los
// tracepoints, envía un SIGKILL controlado a un proceso hijo y verifica que el
// evento llega con el PID y la señal esperados y que la correlación solo se
// confirma con ese evento real (sin falsos positivos).
//
// Requiere privilegios (root/CAP_BPF). Sin ellos, se omite automáticamente.
func TestBPFConsumerIntegracionPrivilegiada(t *testing.T) {
	c, err := NewBPFConsumer()
	if err != nil {
		t.Fatalf("NewBPFConsumer: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := c.Start(ctx); err != nil {
		_ = c.Close()
		if isPrivilegeError(err) {
			t.Skipf("host sin privilegios eBPF (se omite): %v", err)
		}
		t.Fatalf("Start del consumidor real: %v", err)
	}
	defer func() { _ = c.Close() }()

	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("no se pudo lanzar el proceso hijo: %v", err)
	}
	childPID := cmd.Process.Pid
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })

	corr := NewCorrelator(10 * time.Second)
	corr.Track("cont-integration", childPID, 9)
	if corr.PendingCount() != 1 {
		t.Fatalf("pendientes = %d, esperaba 1", corr.PendingCount())
	}

	// Un evento sintético con la señal equivocada NO debe confirmar nada:
	// Valkey solo registra tras el evento real correlacionado.
	if _, err := corr.Confirm(Event{TargetPID: childPID, Signal: 15}); !errors.Is(err, ErrNoCorrelation) {
		t.Fatalf("evento falso (SIGTERM) confirmó: %v", err)
	}

	// Señal real hacia el proceso hijo.
	if err := unix.Kill(childPID, unix.SIGKILL); err != nil {
		t.Fatalf("kill(%d): %v", childPID, err)
	}

	ev, ok := waitForEvent(t, c.Events(), func(e Event) bool {
		return e.TargetPID == childPID && e.Signal == 9
	}, 10*time.Second)
	if !ok {
		t.Fatalf("no llegó el evento eBPF real para pid=%d sig=9", childPID)
	}
	if ev.TimestampNS <= 0 {
		t.Errorf("timestamp inválido: %d", ev.TimestampNS)
	}
	if ev.EmitterPID <= 0 {
		t.Errorf("emitter pid inválido: %d", ev.EmitterPID)
	}

	p, err := corr.Confirm(ev)
	if err != nil {
		t.Fatalf("confirmación del evento real: %v", err)
	}
	if p.ContainerID != "cont-integration" || p.HostPID != childPID {
		t.Errorf("confirmación = %+v", p)
	}
	if corr.PendingCount() != 0 {
		t.Errorf("pendientes = %d, esperaba 0 tras confirmar", corr.PendingCount())
	}
}

// waitForEvent consume el canal de eventos hasta encontrar uno que cumpla el
// predicado o agotar el timeout.
func waitForEvent(t *testing.T, ch <-chan Event, pred func(Event) bool, timeout time.Duration) (Event, bool) {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return Event{}, false
			}
			if pred(ev) {
				return ev, true
			}
		case <-timer.C:
			return Event{}, false
		}
	}
}
