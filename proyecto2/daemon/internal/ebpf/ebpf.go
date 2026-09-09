// Package ebpf define los contratos del consumidor eBPF y la correlación de
// eventos de terminación.
//
// El daemon NO elimina un contenedor hasta confirmarlo con un evento eBPF
// correlacionado. Este paquete expone la interfaz Consumer (que un puente eBPF
// real con ring buffer implementará más adelante) y un Correlator que decide
// si un evento entrante confirma una solicitud de terminación pendiente.
package ebpf

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrNoCorrelation indica que un evento no confirmó ninguna solicitud pendiente.
var ErrNoCorrelation = errors.New("ebpf: evento sin correlación")

// Signal de terminación usado por el daemon (valor IOS/syscall común).
const SignalKill = "SIGKILL"

// Event es un evento de syscall kill observado por eBPF.
type Event struct {
	// EmitterPID es el PID/TGID del proceso que emitió kill.
	EmitterPID int
	// TargetPID es el PID receptor de la señal.
	TargetPID int
	// Signal es la señal emitida (ej. 9 para SIGKILL).
	Signal int
	// TimestampNS es el instante del evento en nanosegundos.
	TimestampNS int64
}

// Consumer abstrae el origen de eventos eBPF (ring buffer / callback).
//
// Un puente CO-RE con libbpf implementará este contrato leyendo el ring buffer
// y emitiendo eventos tipados. El daemon solo depende de esta interfaz, por lo
// que el consumidor real se integra sin tocar el resto del servicio.
type Consumer interface {
	// Events devuelve el canal de eventos confirmables. Debe cerrarse al
	// detener el consumidor.
	Events() <-chan Event
	// Start inicia la lectura del ring buffer / suscripción al tracepoint.
	Start(ctx context.Context) error
	// Close libera los recursos eBPF y cierra el canal de eventos.
	Close() error
}

// Pending es una solicitud de terminación que espera confirmación eBPF.
type Pending struct {
	ContainerID string
	HostPID     int
	Signal      int
	RequestedAt time.Time
}

// Correlator vincula solicitudes de terminación pendientes con los eventos
// eBPF entrantes. Solo un evento que coincide en PID receptor, señal y ventana
// temporal confirma una solicitud.
type Correlator struct {
	mu      sync.Mutex
	pending map[int][]Pending // HostPID -> solicitudes
	window  time.Duration
	now     func() time.Time
}

// NewCorrelator construye un Correlator con la ventana temporal de correlación.
func NewCorrelator(window time.Duration) *Correlator {
	return &Correlator{
		pending: map[int][]Pending{},
		window:  window,
		now:     time.Now,
	}
}

// Track registra una solicitud de terminación pendiente para un PID de host.
func (c *Correlator) Track(containerID string, hostPID, signal int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneLocked()
	c.pending[hostPID] = append(c.pending[hostPID], Pending{
		ContainerID: containerID,
		HostPID:     hostPID,
		Signal:      signal,
		RequestedAt: c.now(),
	})
}

// Confirm procesa un evento eBPF y, si coincide con una solicitud pendiente
// dentro de la ventana temporal, la confirma y la consume.
func (c *Correlator) Confirm(ev Event) (Pending, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneLocked()

	list, ok := c.pending[ev.TargetPID]
	if !ok || len(list) == 0 {
		return Pending{}, ErrNoCorrelation
	}
	// Buscar una solicitud con la misma señal, tomando la más antigua.
	idx := -1
	for i := range list {
		if list[i].Signal == ev.Signal {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Pending{}, ErrNoCorrelation
	}
	p := list[idx]
	list = append(list[:idx], list[idx+1:]...)
	if len(list) == 0 {
		delete(c.pending, ev.TargetPID)
	} else {
		c.pending[ev.TargetPID] = list
	}
	return p, nil
}

// PendingCount devuelve cuántas solicitudes esperan confirmación.
func (c *Correlator) PendingCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneLocked()
	total := 0
	for _, l := range c.pending {
		total += len(l)
	}
	return total
}

// pruneLocked descarta solicitudes fuera de la ventana temporal.
// Requiere que el llamador tenga el mutex tomado.
func (c *Correlator) pruneLocked() {
	cutoff := c.now().Add(-c.window)
	for pid, list := range c.pending {
		kept := list[:0]
		for _, p := range list {
			if !p.RequestedAt.Before(cutoff) {
				kept = append(kept, p)
			}
		}
		if len(kept) == 0 {
			delete(c.pending, pid)
		} else {
			c.pending[pid] = kept
		}
	}
}
