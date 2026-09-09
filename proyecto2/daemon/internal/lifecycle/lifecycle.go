// Package lifecycle orquesta el ciclo de reconciliación del daemon.
//
// Conecta procfs, Docker, política, Valkey y eBPF en un bucle configurable.
// La terminación de un contenedor nunca se confirma en Valkey sin antes
// recibir el evento eBPF correlacionado. El bucle se detiene y libera recursos
// al recibir SIGINT/SIGTERM (contexto cancelado).
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/202300476/p2-daemon/internal/config"
	"github.com/202300476/p2-daemon/internal/docker"
	"github.com/202300476/p2-daemon/internal/ebpf"
	"github.com/202300476/p2-daemon/internal/policy"
	"github.com/202300476/p2-daemon/internal/procfs"
	"github.com/202300476/p2-daemon/internal/valkey"
)

// ErrConsumerFailed indica que el consumidor eBPF no pudo arrancar.
var ErrConsumerFailed = errors.New("lifecycle: consumidor eBPF falló")

// Result resume el desenlace de un ciclo de reconciliación.
type Result struct {
	ProcessCount      int
	ManagedCount      int
	SurvivorCount     int
	RemovalCandidates int
	ConfirmedKills    int
	ExcludedCount     int
}

// Daemon es la unidad de ejecución del ciclo. Agrupa todas las dependencias.
type Daemon struct {
	Cfg     config.Config
	Proc    procfs.Reader
	Docker  docker.ContainerAPI
	Policy  policy.Policy
	Store   valkey.Store
	Events  ebpf.Consumer
	Corr    *ebpf.Correlator
	Logger  *slog.Logger
	Compose ComposeRunner

	// degraded indica que el consumidor eBPF real no pudo cargarse y se opera
	// con un consumidor nulo (EBPFMode=auto). Ninguna eliminación se registrará
	// como confirmada porque no hay eventos de kernel que la confirmen.
	degraded atomic.Bool

	// signalFunc traduce la señal del sistema (SIGKILL) al código de la
	// syscall kill esperado por el correlator.
	SignalFunc func(name string) int

	// ProcessKill envía la señal directamente al PID de host. Esto garantiza
	// que la terminación atraviese kill(2), el tracepoint exigido para la
	// confirmación eBPF. Docker.Kill queda como respaldo si la syscall falla.
	ProcessKill func(pid int, signal syscall.Signal) error
}

// New construye un Daemon con todas sus dependencias.
func New(cfg config.Config, proc procfs.Reader, dc docker.ContainerAPI, pol policy.Policy, st valkey.Store, cons ebpf.Consumer, corr *ebpf.Correlator, logger *slog.Logger) *Daemon {
	return &Daemon{
		Cfg:         cfg,
		Proc:        proc,
		Docker:      dc,
		Policy:      pol,
		Store:       st,
		Events:      cons,
		Corr:        corr,
		Logger:      logger,
		Compose:     NewDockerComposeRunner(logger),
		SignalFunc:  signalNumber,
		ProcessKill: syscall.Kill,
	}
}

// Degraded indica si el daemon opera sin confirmación eBPF real.
func (d *Daemon) Degraded() bool { return d.degraded.Load() }

// Run valida el entorno y ejecuta el bucle de reconciliación hasta que se
// cancela el contexto. Retorna nil en apagado ordenado.
func (d *Daemon) Run(ctx context.Context) error {
	if err := d.Docker.Ping(ctx); err != nil {
		return fmt.Errorf("lifecycle: docker no disponible: %w", err)
	}
	// Valkey/Grafana: si el stack Compose está configurado y Valkey no responde,
	// se levanta el stack del proyecto (no toca servicios ajenos) y se reintenta.
	if err := ComposeBootstrap(ctx, d.Cfg.ComposeFile, d.Compose, d.Store, d.Logger, 30*time.Second); err != nil {
		if ctx.Err() != nil {
			// Apagado ordenado recibido durante el bootstrap.
			d.Logger.Info("apagado durante bootstrap de Compose")
			return nil
		}
		return err
	}
	if err := d.startConsumer(ctx); err != nil {
		return err
	}
	defer func() { _ = d.Events.Close() }()

	go d.runEventConsumer(ctx)

	ticker := time.NewTicker(d.Cfg.LoopInterval)
	defer ticker.Stop()

	if _, err := d.reconcile(ctx); err != nil {
		d.Logger.Error("ciclo inicial fallido", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			d.Logger.Info("apagado por señal, deteniendo loop")
			return nil
		case <-ticker.C:
			if _, err := d.reconcile(ctx); err != nil {
				d.Logger.Error("ciclo de reconciliación fallido", "error", err)
			}
		}
	}
}

// startConsumer arranca el consumidor eBPF según el modo configurado:
//   - off:      consumidor nulo explícito (sin eventos).
//   - required: el puente CO-RE real es obligatorio; sin privilegios el
//     daemon falla con un error accionable.
//   - auto:     intenta el puente real; si el entorno no concede privilegios,
//     degrada a un consumidor nulo con un mensaje claro.
func (d *Daemon) startConsumer(ctx context.Context) error {
	if d.Cfg.EBPFMode == "off" {
		d.Logger.Warn("eBPF desactivado por configuración (EBPFMode=off); no habrá confirmación de eliminaciones")
		_ = d.Events.Close()
		d.Events = ebpf.NewNullConsumer()
		d.degraded.Store(true)
		return nil
	}
	if err := d.Events.Start(ctx); err != nil {
		if d.Cfg.EBPFMode == "auto" && ebpf.IsPrivilegeError(err) {
			d.Logger.Warn("eBPF no disponible sin privilegios (root/CAP_BPF); degradando a consumidor nulo", "error", err)
			_ = d.Events.Close()
			d.Events = ebpf.NewNullConsumer()
			d.degraded.Store(true)
			return nil
		}
		return fmt.Errorf("%w: %v (requiere root/CAP_BPF; use SO1_P2_EBPF_MODE=off para degradar explícitamente)", ErrConsumerFailed, err)
	}
	return nil
}

// runEventConsumer lee el canal de eventos eBPF y confirma eliminaciones
// correlacionadas, registrándolas en Valkey. Es el único punto donde un kill
// se marca como confirmado.
func (d *Daemon) runEventConsumer(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-d.Events.Events():
			if !ok {
				return
			}
			p, err := d.Corr.Confirm(ev)
			if errors.Is(err, ebpf.ErrNoCorrelation) {
				d.Logger.Debug("evento eBPF sin correlación", "target_pid", ev.TargetPID, "signal", ev.Signal)
				continue
			}
			if err != nil {
				d.Logger.Warn("error correlacionando evento", "error", err)
				continue
			}
			d.Logger.Info("eliminación confirmada por eBPF", "container", p.ContainerID, "host_pid", p.HostPID)
			killEv := valkey.KillEvent{
				ContainerID:     p.ContainerID,
				HostPID:         p.HostPID,
				Signal:          d.Cfg.KillSignal,
				TimestampUnixNS: time.Now().UnixNano(),
			}
			if err := d.Store.RecordKill(ctx, killEv); err != nil {
				d.Logger.Error("falló el registro de kill confirmado", "error", err)
			}
		}
	}
}

// reconcile ejecuta un ciclo completo de reconciliación.
func (d *Daemon) reconcile(ctx context.Context) (Result, error) {
	var res Result

	data, err := d.Proc.Read()
	if err != nil {
		return res, fmt.Errorf("lifecycle: lectura de /proc: %w", err)
	}
	snap, err := procfs.Parse(data)
	if err != nil {
		return res, fmt.Errorf("lifecycle: parseo de snapshot: %w", err)
	}
	res.ProcessCount = len(snap.Processes)

	managed, err := d.Docker.ListManaged(ctx, d.Cfg.ManagedLabelKey, d.Cfg.ManagedLabelValue)
	if err != nil {
		return res, fmt.Errorf("lifecycle: listado docker: %w", err)
	}

	enricher := snapshotEnricher{snap: snap}
	var enriched []docker.Container
	for _, m := range managed {
		insp, err := d.Docker.Inspect(ctx, m.ID)
		if err != nil {
			d.Logger.Warn("inspect fallido", "id", m.ID, "error", err)
			continue
		}
		if insp.HostPID > 0 {
			insp.MemoryKB = 0
			insp.CPU = 0
		}
		enriched = append(enriched, insp)
	}
	res.ManagedCount = len(enriched)

	dec := d.Policy.Decide(enriched, enricher)
	res.SurvivorCount = len(dec.Survivors)
	res.RemovalCandidates = len(dec.RemovalCandidates)
	res.ExcludedCount = len(dec.Excluded)

	// Persistencia histórica.
	sysEntry := valkey.SnapshotEntry{
		TimestampUnixNS: snap.TimestampUnixNS,
		MemTotalKB:      snap.Memory.TotalKB,
		MemFreeKB:       snap.Memory.FreeKB,
		MemUsedKB:       snap.Memory.UsedKB,
		ProcessCount:    len(snap.Processes),
	}
	if err := d.Store.SaveSystemLatest(ctx, sysEntry); err != nil {
		d.Logger.Warn("no se pudo guardar system:latest", "error", err)
	}
	if err := d.Store.AppendSystemTimeline(ctx, sysEntry); err != nil {
		d.Logger.Warn("no se pudo guardar system:timeline", "error", err)
	}

	hist := make([]valkey.ContainerEntry, 0, len(dec.Survivors)+len(dec.RemovalCandidates))
	for _, c := range append(append([]policy.Candidate{}, dec.Survivors...), dec.RemovalCandidates...) {
		hist = append(hist, valkey.ContainerEntry{
			ContainerID:     c.Container.ID,
			Name:            c.Container.DisplayName(),
			HostPID:         c.Container.HostPID,
			MemPercent:      c.MemPercent,
			VSZKB:           c.VSZKB,
			RSSKB:           c.RSSKB,
			CPU:             c.CPU,
			TimestampUnixNS: snap.TimestampUnixNS,
		})
	}
	if err := d.Store.AppendContainersTimeline(ctx, hist); err != nil {
		d.Logger.Warn("no se pudo guardar containers:timeline", "error", err)
	}
	if err := d.Store.AddRankings(ctx, hist); err != nil {
		d.Logger.Warn("no se pudieron guardar rankings", "error", err)
	}

	// Solicitar terminación de los excedentes. La confirmación llega por eBPF.
	sigNum := d.SignalFunc(d.Cfg.KillSignal)
	for _, c := range dec.RemovalCandidates {
		d.Corr.Track(c.Container.ID, c.Container.HostPID, sigNum)
		d.Logger.Info("solicitando terminación", "container", c.Container.DisplayName(), "host_pid", c.Container.HostPID)
		if err := d.ProcessKill(c.Container.HostPID, syscall.Signal(sigNum)); err != nil {
			d.Logger.Warn("falló kill(2); usando respaldo de Docker", "container", c.Container.ID, "host_pid", c.Container.HostPID, "error", err)
			if dockerErr := d.Docker.Kill(ctx, c.Container.ID, d.Cfg.KillSignal); dockerErr != nil {
				d.Logger.Warn("falló la solicitud de terminación", "container", c.Container.ID, "error", dockerErr)
			}
		}
	}

	return res, nil
}

// snapshotEnricher implementa policy.Enricher buscando un PID de host en la
// última instantánea del módulo /proc.
type snapshotEnricher struct {
	snap procfs.Snapshot
}

// Enrich devuelve las métricas del proceso asociado al PID de host.
func (e snapshotEnricher) Enrich(hostPID int) (float64, float64, float64, float64, bool) {
	p, ok := e.snap.FindByPID(int64(hostPID))
	if !ok {
		return 0, 0, 0, 0, false
	}
	return p.MemoryPercent, float64(p.VSZKB), float64(p.RSSKB), p.CPUPercent, true
}

// signalNumber traduce el nombre de una señal (ej. SIGKILL) a su número.
// Se usa para correlacionar con el campo Signal de los eventos eBPF.
func signalNumber(name string) int {
	switch name {
	case "SIGKILL", "9":
		return 9
	case "SIGTERM", "15":
		return 15
	case "SIGINT", "2":
		return 2
	default:
		return 9
	}
}
