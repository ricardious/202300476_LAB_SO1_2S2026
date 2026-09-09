// Package lifecycle (módulo de kernel) — ciclo de vida real del módulo /proc.
//
// El daemon delega la carga/descarga en los scripts idempotentes
// proyecto2/scripts/{load,unload}-module.sh, que re-ejecutan con sudo cuando
// hace falta y verifican la creación/eliminación de /proc antes de terminar.
// La ruta de cada script es configurable (SO1_P2_MODULE_{LOAD,UNLOAD}_SCRIPT)
// para permitir instalaciones en otras rutas del sistema.
package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
)

// ModuleExecutor ejecuta un script de ciclo de vida (inyectable en pruebas).
type ModuleExecutor func(ctx context.Context, path string) error

// ModuleManager gestiona la carga y descarga del módulo de kernel.
type ModuleManager struct {
	LoadScript   string
	UnloadScript string
	Logger       *slog.Logger
	exec         ModuleExecutor
}

// NewModuleManager construye un ModuleManager con el executor del sistema.
func NewModuleManager(loadScript, unloadScript string, logger *slog.Logger) *ModuleManager {
	return &ModuleManager{
		LoadScript:   loadScript,
		UnloadScript: unloadScript,
		Logger:       logger,
		exec:         runModuleScript,
	}
}

// NewModuleManagerWithExecutor construye un ModuleManager con executor propio
// (dobles en pruebas).
func NewModuleManagerWithExecutor(loadScript, unloadScript string, logger *slog.Logger, exec ModuleExecutor) *ModuleManager {
	return &ModuleManager{
		LoadScript:   loadScript,
		UnloadScript: unloadScript,
		Logger:       logger,
		exec:         exec,
	}
}

// Load ejecuta el script de carga (compila y hace insmod si hace falta).
// Es idempotente: si el módulo ya está cargado, el script no hace nada.
func (m *ModuleManager) Load(ctx context.Context) error {
	if m.LoadScript == "" {
		return nil
	}
	if m.Logger != nil {
		m.Logger.Info("cargando módulo de kernel", "script", m.LoadScript)
	}
	if err := m.exec(ctx, m.LoadScript); err != nil {
		return fmt.Errorf("lifecycle: falló la carga del módulo (%s): %w", m.LoadScript, err)
	}
	return nil
}

// Unload ejecuta el script de descarga (rmmod si hace falta). Es idempotente.
func (m *ModuleManager) Unload(ctx context.Context) error {
	if m.UnloadScript == "" {
		return nil
	}
	if m.Logger != nil {
		m.Logger.Info("descargando módulo de kernel", "script", m.UnloadScript)
	}
	if err := m.exec(ctx, m.UnloadScript); err != nil {
		return fmt.Errorf("lifecycle: falló la descarga del módulo (%s): %w", m.UnloadScript, err)
	}
	return nil
}

// runModuleScript ejecuta un script con bash, propagando stdout/stderr para
// que los mensajes de los scripts (idempotencia, errores) sean visibles.
func runModuleScript(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "bash", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
