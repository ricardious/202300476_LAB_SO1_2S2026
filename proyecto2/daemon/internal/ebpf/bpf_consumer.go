package ebpf

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@v0.22.0 -cc clang -target bpf killmonitor ../../../ebpf/kill_monitor.bpf.c

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
)

// ringEvent es la vista binaria del struct kill_event definido en
// proyecto2/ebpf/kill_monitor.h: timestamp_ns, emitter_pid, target_pid,
// signal y reserved (24 bytes, alineación 8).
type ringEvent struct {
	TimestampNS uint64
	EmitterPID  int32
	TargetPID   int32
	Signal      int32
	Reserved    uint32
}

// ringEventSize es el tamaño fijo de un evento emitido por el programa eBPF.
const ringEventSize = int(unsafe.Sizeof(ringEvent{}))

// decodeRingEvent convierte la muestra cruda del ring buffer en un Event
// tipado del contrato del paquete. El layout binario coincide 1:1 con el
// struct C kill_event.
func decodeRingEvent(raw []byte) (Event, error) {
	if len(raw) != ringEventSize {
		return Event{}, fmt.Errorf("ebpf: tamaño de evento %d, esperaba %d", len(raw), ringEventSize)
	}
	// Copia a un buffer alineado local para evitar requisitos de alineación
	// sobre la muestra del ring buffer.
	var buf [24]byte
	copy(buf[:], raw)
	e := (*ringEvent)(unsafe.Pointer(&buf[0]))
	return Event{
		EmitterPID:  int(e.EmitterPID),
		TargetPID:   int(e.TargetPID),
		Signal:      int(e.Signal),
		TimestampNS: int64(e.TimestampNS),
	}, nil
}

// tracepointTargets son los puntos de adjunto del monitor. El primario es
// sys_enter_kill (lo que usa Docker/runc); tgkill amplía la cobertura a
// señales dirigidas por hilo.
var tracepointTargets = []struct {
	group string
	name  string
	prog  func(*BPFConsumer) *ebpf.Program
}{
	{"syscalls", "sys_enter_kill", func(c *BPFConsumer) *ebpf.Program { return c.objs.HandleKill }},
	{"syscalls", "sys_enter_tgkill", func(c *BPFConsumer) *ebpf.Program { return c.objs.HandleTgkill }},
}

// BPFConsumer es un consumidor real del ring buffer eBPF. Implementa
// ebpf.Consumer: carga el objeto CO-RE compilado (killmonitor_bpf.o),
// adjunta los tracepoints sys_enter_kill/tgkill y reenvía cada evento
// tipado al canal Events(). Close libera todos los recursos.
type BPFConsumer struct {
	spec *ebpf.CollectionSpec

	mu          sync.Mutex
	started     bool
	closed      bool
	objs        killmonitorObjects
	links       []link.Link
	rd          *ringbuf.Reader
	events      chan Event
	done        chan struct{}
	closeEvents sync.Once
	releaseOnce sync.Once
	releaseErr  error
}

// NewBPFConsumer construye un BPFConsumer parseando el objeto eBPF embebido.
// No requiere privilegios: la carga al kernel se realiza en Start.
func NewBPFConsumer() (*BPFConsumer, error) {
	spec, err := loadKillmonitor()
	if err != nil {
		return nil, fmt.Errorf("ebpf: especificación inválida: %w", err)
	}
	for _, prog := range []string{killmonitorProgHandleKill, killmonitorProgHandleTgkill} {
		if spec.Programs[prog] == nil {
			return nil, fmt.Errorf("ebpf: programa %q ausente en el objeto", prog)
		}
	}
	if spec.Maps[killmonitorMapEvents] == nil {
		return nil, fmt.Errorf("ebpf: mapa %q ausente en el objeto", killmonitorMapEvents)
	}
	return &BPFConsumer{
		spec:   spec,
		events: make(chan Event, 1024),
		done:   make(chan struct{}),
	}, nil
}

// Events implementa ebpf.Consumer.Events. Se cierra al detener el consumidor.
func (c *BPFConsumer) Events() <-chan Event { return c.events }

// Start implementa ebpf.Consumer.Start: carga el objeto CO-RE, crea el lector
// del ring buffer, adjunta los tracepoints y comienza a reenviar eventos.
func (c *BPFConsumer) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return errors.New("ebpf: consumidor cerrado")
	}
	if c.started {
		c.mu.Unlock()
		return errors.New("ebpf: consumidor ya iniciado")
	}
	c.mu.Unlock()

	// Best-effort: kernels viejos usan RLIMIT_MEMLOCK para mapas.
	if err := rlimit.RemoveMemlock(); err != nil {
		_ = err // no bloqueante en kernels con contabilidad por memcg
	}

	if err := loadKillmonitorObjects(&c.objs, nil); err != nil {
		return fmt.Errorf("ebpf: carga de objetos CO-RE: %w", err)
	}

	rd, err := ringbuf.NewReader(c.objs.Events)
	if err != nil {
		_ = c.release()
		return fmt.Errorf("ebpf: lector de ring buffer: %w", err)
	}
	c.rd = rd

	for _, tp := range tracepointTargets {
		l, err := link.Tracepoint(tp.group, tp.name, tp.prog(c), nil)
		if err != nil {
			_ = c.release()
			return fmt.Errorf("ebpf: adjuntar %s/%s: %w", tp.group, tp.name, err)
		}
		c.links = append(c.links, l)
	}

	c.mu.Lock()
	c.started = true
	c.mu.Unlock()

	go c.run(ctx)
	return nil
}

// run lee el ring buffer y reenvía los eventos al canal hasta que se cierra
// el lector o el contexto se cancela. Al salir, cierra el canal de eventos.
func (c *BPFConsumer) run(ctx context.Context) {
	defer c.closeEvents.Do(func() { close(c.events) })
	for {
		rec, err := c.rd.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) || ctx.Err() != nil {
				return
			}
			continue
		}
		ev, derr := decodeRingEvent(rec.RawSample)
		if derr != nil {
			continue
		}
		select {
		case c.events <- ev:
		case <-c.done:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Close implementa ebpf.Consumer.Close: cierra el lector, desadjunta los
// tracepoints, libera el objeto y cierra el canal de eventos.
func (c *BPFConsumer) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	started := c.started
	c.mu.Unlock()

	close(c.done)
	if !started {
		// Sin goroutine: el canal se cierra aquí.
		c.closeEvents.Do(func() { close(c.events) })
	}
	return c.release()
}

// release cierra lector, links y objetos eBPF una sola vez.
func (c *BPFConsumer) release() error {
	c.releaseOnce.Do(func() {
		var errs []error
		if c.rd != nil {
			if err := c.rd.Close(); err != nil {
				errs = append(errs, err)
			}
		}
		for _, l := range c.links {
			if err := l.Close(); err != nil {
				errs = append(errs, err)
			}
		}
		// Close() de cilium/ebpf es nil-safe: si el objeto nunca se cargó,
		// no hace nada.
		if err := c.objs.Close(); err != nil {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			c.releaseErr = errors.Join(errs...)
		}
	})
	return c.releaseErr
}
