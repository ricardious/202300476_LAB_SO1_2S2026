// Daemon de gestión autónoma de contenedores del proyecto 2 de SO1.
//
// Conecta configuración, lectura de /proc, Docker, política, Valkey, eBPF y
// cron; controla señales SIGINT/SIGTERM para un apagado ordenado.
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
	cons := ebpf.NewNullConsumer() // el puente CO-RE real se integra aquí
	corr := ebpf.NewCorrelator(cfg.ConfirmationTimeout)

	daemon := lifecycle.New(cfg, proc, dc, pol, store, cons, corr, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Cronjob del proyecto: idempotente y retirado al apagar.
	cr := cron.NewCrontabManager()
	entry := cron.Entry{
		Schedule: "*/2 * * * *",
		Command:  "/usr/local/bin/so1-p2-generate-containers.sh",
	}
	if err := cr.Install(ctx, entry); err != nil {
		logger.Warn("no se pudo instalar el cronjob", "error", err)
	}

	if cfg.LoadModuleOnStart {
		logger.Info("cargando módulo /proc (hooks de despliegue)")
	}

	logger.Info("daemon iniciado", "proc_path", cfg.ProcPath, "loop", cfg.LoopInterval.String(), "min_low", cfg.MinLow, "min_high", cfg.MinHigh)

	if err := daemon.Run(ctx); err != nil {
		logger.Error("daemon terminó con error", "error", err)
	}

	// Apagado ordenado: retirar únicamente la entrada del cron del proyecto.
	if err := cr.Remove(ctx); err != nil {
		logger.Warn("no se pudo retirar el cronjob", "error", err)
	}
	if cfg.UnloadModuleOnStop {
		logger.Info("descargando módulo /proc")
	}
	logger.Info("daemon detenido")
}
