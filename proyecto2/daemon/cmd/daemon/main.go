// Daemon de gestión autónoma de contenedores del proyecto 2 de SO1.
//
// Conecta configuración, lectura de /proc, Docker, política, Valkey, eBPF y
// cron; controla señales SIGINT/SIGTERM para un apagado ordenado.
//
// Flujo de inicio (PLAN_PROYECTO2.md §3.2):
//  1. Validar configuración y construir dependencias.
//  2. Cargar el módulo /proc (script idempotente) si LoadModuleOnStart.
//  3. Instalar el cronjob del proyecto (delimitado, sin tocar entradas ajenas).
//  4. Run: verifica Docker, levanta Valkey/Grafana vía Compose si hace falta,
//     arranca el consumidor eBPF (CO-RE real; degrada solo si EBPFMode=auto y
//     el entorno no tiene privilegios) y ejecuta el loop de reconciliación.
//
// Apagado: retira solo la entrada de cron, descarga el módulo si
// UnloadModuleOnStop y (opcionalmente) baja Compose del proyecto.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/202300476/p2-daemon/internal/config"
	"github.com/202300476/p2-daemon/internal/cron"
	"github.com/202300476/p2-daemon/internal/docker"
	"github.com/202300476/p2-daemon/internal/ebpf"
	"github.com/202300476/p2-daemon/internal/lifecycle"
	"github.com/202300476/p2-daemon/internal/policy"
	"github.com/202300476/p2-daemon/internal/procfs"
	"github.com/202300476/p2-daemon/internal/valkey"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg, err := config.FromEnv()
	if err != nil {
		logger.Error("configuración inválida", "error", err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		logger.Error("validación de configuración fallida", "error", err)
		os.Exit(1)
	}

	// Dependencias de infraestructura.
	dc, err := docker.New(cfg.DockerHost, cfg.DockerAPIVersion)
	if err != nil {
		logger.Error("no se pudo construir el cliente docker", "error", err)
		os.Exit(1)
	}
	store := valkey.New(cfg.ValkeyAddr, cfg.ValkeyPassword, cfg.ValkeyDB, cfg.KeyPrefix)

	proc := procfs.FileReader{Path: cfg.ProcPath}
	pol := policy.New(cfg)

	// Consumidor eBPF real (CO-RE). La degradación a consumidor nulo solo
	// ocurre dentro de lifecycle.Run cuando EBPFMode=auto y el entorno no
	// concede privilegios; con EBPFMode=off se desactiva explícitamente.
	var cons ebpf.Consumer
	if cfg.EBPFMode == "off" {
		logger.Warn("eBPF desactivado por configuración (SO1_P2_EBPF_MODE=off)")
		cons = ebpf.NewNullConsumer()
	} else {
		realCons, err := ebpf.NewBPFConsumer()
		if err != nil {
			logger.Error("no se pudo construir el consumidor eBPF CO-RE", "error", err)
			os.Exit(1)
		}
		cons = realCons
	}
	corr := ebpf.NewCorrelator(cfg.ConfirmationTimeout)

	daemon := lifecycle.New(cfg, proc, dc, pol, store, cons, corr, logger)

	// Ciclo de vida real del módulo de kernel (scripts idempotentes).
	mod := lifecycle.NewModuleManager(cfg.ModuleLoadScript, cfg.ModuleUnloadScript, logger)
	if cfg.LoadModuleOnStart {
		if err := mod.Load(context.Background()); err != nil {
			logger.Error("no se pudo cargar el módulo /proc", "error", err)
			os.Exit(1)
		}
	} else {
		logger.Info("carga del módulo /proc omitida por configuración")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Cronjob del proyecto: idempotente y retirado al apagar.
	cr := cron.NewCrontabManager()
	entry := cron.Entry{
		Schedule: cfg.CronSchedule,
		Command:  cfg.CronCommand,
	}
	if err := cr.Install(ctx, entry); err != nil {
		logger.Warn("no se pudo instalar el cronjob", "error", err)
	}

	logger.Info("daemon iniciado", "proc_path", cfg.ProcPath, "loop", cfg.LoopInterval.String(), "min_low", cfg.MinLow, "min_high", cfg.MinHigh, "ebpf_mode", cfg.EBPFMode)

	var runErr error
	if err := daemon.Run(ctx); err != nil {
		logger.Error("daemon terminó con error", "error", err)
		runErr = err
	}
	if daemon.Degraded() {
		logger.Warn("el daemon operó SIN confirmación eBPF (consumidor nulo); ninguna eliminación se registró como confirmada")
	}

	// Apagado ordenado: retirar únicamente la entrada del cron del proyecto.
	if err := cr.Remove(ctx); err != nil {
		logger.Warn("no se pudo retirar el cronjob", "error", err)
	}
	if cfg.UnloadModuleOnStop {
		if err := mod.Unload(context.Background()); err != nil {
			logger.Warn("no se pudo descargar el módulo /proc", "error", err)
		}
	} else {
		logger.Info("descarga del módulo /proc omitida por configuración")
	}
	if cfg.ComposeDownOnStop && cfg.ComposeFile != "" {
		runner := lifecycle.NewDockerComposeRunner(logger)
		if err := runner.Down(ctx, cfg.ComposeFile); err != nil {
			logger.Warn("no se pudo bajar el stack Compose", "error", err)
		}
	}
	logger.Info("daemon detenido")
	if runErr != nil {
		os.Exit(1)
	}
}
