package ebpf

import (
	"context"
	"sync"
)

// NullConsumer es un consumidor inerte que nunca emite eventos. Útil como
// default hasta integrar el puente CO-RE real.
type NullConsumer struct {
	started bool
	closed  bool
	mu      sync.Mutex
	ch      chan Event
}

// NewNullConsumer construye un NullConsumer.
func NewNullConsumer() *NullConsumer {
	return &NullConsumer{ch: make(chan Event)}
}

// Events implementa Consumer.Events.
func (n *NullConsumer) Events() <-chan Event { return n.ch }

// Start implementa Consumer.Start.
func (n *NullConsumer) Start(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.closed {
		return context.Canceled
	}
	n.started = true
	return nil
}

// Close implementa Consumer.Close.
func (n *NullConsumer) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.closed {
		close(n.ch)
		n.closed = true
	}
	return nil
}

// MemConsumer es un consumidor en memoria para pruebas y demostraciones:
// emite eventos escritos programáticamente por el canal.
type MemConsumer struct {
	ch   chan Event
	once sync.Once
}

// NewMemConsumer construye un MemConsumer con un canal con buffer.
func NewMemConsumer(buf int) *MemConsumer {
	if buf < 1 {
		buf = 1
	}
	return &MemConsumer{ch: make(chan Event, buf)}
}

// Events implementa Consumer.Events.
func (m *MemConsumer) Events() <-chan Event { return m.ch }

// Start implementa Consumer.Start (no requiere arranque).
func (m *MemConsumer) Start(ctx context.Context) error { return nil }

// Close implementa Consumer.Close cerrando el canal una vez.
func (m *MemConsumer) Close() error {
	m.once.Do(func() { close(m.ch) })
	return nil
}

// Emit publica un evento en el consumidor (bloqueante si el canal está lleno).
func (m *MemConsumer) Emit(ev Event) error {
	select {
	case m.ch <- ev:
		return nil
	default:
		return ErrNoCorrelation
	}
}
