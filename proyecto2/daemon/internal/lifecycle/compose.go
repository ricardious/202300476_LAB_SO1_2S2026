// Compose (orquestación de servicios) — arranque reproducible de Valkey/Grafana.
//
// El daemon corre en el HOST (necesita /proc del módulo y eBPF con privilegios)
// mientras Compose ejecuta los servicios de observabilidad. Si Valkey no está
// disponible al arrancar, el daemon intenta levantar el stack Compose del
// proyecto (idempotente: `up -d` no toca servicios ajenos) y reintenta el ping.
package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"
)

// ComposeRunner abstrae el comando `docker compose` (inyectable en pruebas).
type ComposeRunner interface {
	Up(ctx context.Context, file string) error
	Down(ctx context.Context, file string) error
}

// DockerComposeRunner implementa ComposeRunner sobre el binario docker.
type DockerComposeRunner struct {
	Logger *slog.Logger
}

// NewDockerComposeRunner construye un DockerComposeRunner.
func NewDockerComposeRunner(logger *slog.Logger) *DockerComposeRunner {
	return &DockerComposeRunner{Logger: logger}
}

// Up ejecuta `docker compose -f <file> up -d --wait` (--wait respeta los
// healthchecks; si la versión de Compose no soporta --wait, reintenta sin él).
func (r *DockerComposeRunner) Up(ctx context.Context, file string) error {
	if err := composeRun(ctx, file, "--wait", "up", "-d"); err == nil {
		return nil
	}
	return composeRun(ctx, file, "up", "-d")
}

// Down ejecuta `docker compose -f <file> down` sin eliminar volúmenes (los
// datos de Valkey/Grafana se conservan).
func (r *DockerComposeRunner) Down(ctx context.Context, file string) error {
	return composeRun(ctx, file, "down")
}

// composeRun ejecuta docker compose con los argumentos dados.
func composeRun(ctx context.Context, file string, args ...string) error {
	cmdArgs := append([]string{"compose", "-f", file}, args...)
	cmd := exec.CommandContext(ctx, "docker", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ComposeBootstrap asegura que el stack Compose del proyecto esté arriba.
// Devuelve nil si Valkey ya responde o si no hay archivo Compose configurado.
// El parámetro wait es el tiempo total de reintentos tras `up`.
func ComposeBootstrap(ctx context.Context, composeFile string, runner ComposeRunner, store pinger, logger *slog.Logger, wait time.Duration) error {
	if composeFile == "" {
		return nil
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	err := store.Ping(pingCtx)
	cancel()
	if err == nil {
		return nil // Valkey ya estaba disponible
	}
	if logger != nil {
		logger.Warn("Valkey no disponible; levantando stack Compose del proyecto", "file", composeFile, "error", err)
	}
	if runner == nil {
		runner = NewDockerComposeRunner(logger)
	}
	if uerr := runner.Up(ctx, composeFile); uerr != nil {
		return fmt.Errorf("lifecycle: compose up: %w", uerr)
	}
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := store.Ping(pingCtx)
		cancel()
		if err == nil {
			if logger != nil {
				logger.Info("Valkey/Grafana disponibles tras levantar Compose")
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("lifecycle: Valkey sigue sin responder tras levantar Compose: %w", err)
}

// pinger abstrae el ping de Valkey (Store cumple el contrato).
type pinger interface {
	Ping(ctx context.Context) error
}
