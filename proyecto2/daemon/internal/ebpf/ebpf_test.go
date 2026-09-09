package ebpf

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
	"time"
)

func TestIsPrivilegeError(t *testing.T) {
	if !IsPrivilegeError(syscall.EPERM) {
		t.Error("EPERM debe ser error de privilegios")
	}
	if !IsPrivilegeError(fmt.Errorf("ebpf: adjuntar: %w", syscall.EACCES)) {
		t.Error("EACCES envuelto debe detectarse")
	}
	if !IsPrivilegeError(errors.Join(syscall.EPERM, errors.New("extra"))) {
		t.Error("errors.Join con EPERM debe detectarse")
	}
	if IsPrivilegeError(errors.New("otra cosa")) {
		t.Error("error no relacionado no debe ser de privilegios")
	}
	if IsPrivilegeError(nil) {
		t.Error("nil no debe ser error de privilegios")
	}
}

func TestCorrelatorConfirmsMatching(t *testing.T) {
	c := NewCorrelator(time.Minute)
	c.Track("cont-a", 120, 9)

	p, err := c.Confirm(Event{TargetPID: 120, Signal: 9, TimestampNS: 1})
	if err != nil {
		t.Fatalf("esperaba confirmación, obtuve: %v", err)
	}
	if p.ContainerID != "cont-a" || p.HostPID != 120 {
		t.Errorf("confirmación = %+v", p)
	}
	if c.PendingCount() != 0 {
		t.Errorf("pendientes = %d, esperaba 0", c.PendingCount())
	}
}

func TestCorrelatorNoMatch(t *testing.T) {
	c := NewCorrelator(time.Minute)
	c.Track("cont-a", 120, 9)

	if _, err := c.Confirm(Event{TargetPID: 999, Signal: 9}); err != ErrNoCorrelation {
		t.Errorf("PID distinto debe dar ErrNoCorrelation, obtuve %v", err)
	}
	if _, err := c.Confirm(Event{TargetPID: 120, Signal: 15}); err != ErrNoCorrelation {
		t.Errorf("señal distinta debe dar ErrNoCorrelation, obtuve %v", err)
	}
	if c.PendingCount() != 1 {
		t.Errorf("pendientes = %d, esperaba 1 (no se consumió)", c.PendingCount())
	}
}

func TestCorrelatorPrunesExpired(t *testing.T) {
	c := NewCorrelator(time.Millisecond * 10)
	c.now = func() time.Time { return time.Unix(100, 0) }
	c.Track("cont-a", 120, 9)
	// Avanzar el reloj más allá de la ventana.
	c.now = func() time.Time { return time.Unix(100, int64(100*time.Millisecond)).Add(time.Second) }

	if _, err := c.Confirm(Event{TargetPID: 120, Signal: 9}); err != ErrNoCorrelation {
		t.Errorf("solicitud expirada debe dar ErrNoCorrelation, obtuve %v", err)
	}
	if c.PendingCount() != 0 {
		t.Errorf("pendientes = %d, esperaba 0 tras expirar", c.PendingCount())
	}
}

func TestCorrelatorFIFO(t *testing.T) {
	c := NewCorrelator(time.Minute)
	c.Track("cont-first", 120, 9)
	c.Track("cont-second", 120, 9)

	p, err := c.Confirm(Event{TargetPID: 120, Signal: 9})
	if err != nil {
		t.Fatalf("confirmación: %v", err)
	}
	if p.ContainerID != "cont-first" {
		t.Errorf("FIFO: primero confirmado = %s, esperaba cont-first", p.ContainerID)
	}
	if c.PendingCount() != 1 {
		t.Errorf("pendientes = %d, esperaba 1", c.PendingCount())
	}
}

func TestMemConsumerEmitClose(t *testing.T) {
	cons := NewMemConsumer(1)
	if err := cons.Start(nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	ev := Event{TargetPID: 42, Signal: 9}
	if err := cons.Emit(ev); err != nil {
		t.Fatalf("emit: %v", err)
	}
	got := <-cons.Events()
	if got.TargetPID != 42 || got.Signal != 9 {
		t.Errorf("evento = %+v", got)
	}
	if err := cons.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, ok := <-cons.Events(); ok {
		t.Error("canal debe cerrarse tras Close")
	}
}

func TestNullConsumer(t *testing.T) {
	cons := NewNullConsumer()
	if err := cons.Start(nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := cons.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}
