// Package cron gestiona el cronjob del proyecto de forma idempotente.
//
// La entrada del proyecto se delimita con marcadores para poder retirarla sin
// borrar entradas ajenas. La abstracción Manager permite probar la lógica sin
// depender de un demonio crontab real.
package cron

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Markers delimitan el bloque del cronjob del proyecto. Deben coincidir
// EXACTAMENTE con los marcadores de scripts/install-cron.sh y
// scripts/remove-cron.sh para que el daemon Go y los scripts shell operen
// sobre la misma entrada de forma intercambiable.
const (
	BeginMarker = "# >>> so1-p2-202300476:begin >>>"
	EndMarker   = "# <<< so1-p2-202300476:end <<<"
)

// Entry describe un cronjob a instalar.
type Entry struct {
	Schedule string // expresión crontab (ej. "*/2 * * * *")
	Command  string // comando a ejecutar
}

// Manager es el contrato de instalación/retiro del cronjob.
type Manager interface {
	Install(ctx context.Context, e Entry) error
	Remove(ctx context.Context) error
	IsInstalled(ctx context.Context) (bool, error)
}

// ErrExec indica un fallo al ejecutar el comando crontab.
var ErrExec = errors.New("cron: error ejecutando crontab")

// Executor ejecuta un comando con entrada estándar (inyectable en pruebas).
type Executor func(name string, args []string, stdin string) ([]byte, error)

// CrontabManager manipula el crontab del usuario.
type CrontabManager struct {
	Exec Executor
}

// NewCrontabManager construye un CrontabManager con el executor del sistema.
func NewCrontabManager() *CrontabManager {
	return &CrontabManager{Exec: systemCrontab}
}

// installBlock formatea la entrada delimitada por marcadores.
func installBlock(schedule, command string) string {
	return fmt.Sprintf("%s\n%s %s\n%s", BeginMarker, schedule, command, EndMarker)
}

// Install agrega la entrada del proyecto si no está presente (idempotente).
func (m *CrontabManager) Install(ctx context.Context, e Entry) error {
	current, err := m.read(ctx)
	if err != nil {
		return err
	}
	if strings.Contains(current, BeginMarker) {
		return nil // ya instalado
	}
	next := strings.TrimRight(current, "\n") + "\n" + installBlock(e.Schedule, e.Command) + "\n"
	return m.write(ctx, next)
}

// Remove retira únicamente el bloque del proyecto.
func (m *CrontabManager) Remove(ctx context.Context) error {
	current, err := m.read(ctx)
	if err != nil {
		return err
	}
	start := strings.Index(current, BeginMarker)
	if start < 0 {
		return nil
	}
	end := strings.Index(current[start:], EndMarker)
	if end < 0 {
		// Marcador de apertura sin cierre: se corta desde la apertura.
		return m.write(ctx, strings.TrimRight(current[:start], "\n"))
	}
	end += start + len(EndMarker)
	next := strings.Trim(current[:start]+current[end:], "\n")
	return m.write(ctx, next)
}

// IsInstalled indica si el cronjob del proyecto está presente.
func (m *CrontabManager) IsInstalled(ctx context.Context) (bool, error) {
	current, err := m.read(ctx)
	if err != nil {
		return false, err
	}
	return strings.Contains(current, BeginMarker), nil
}

func (m *CrontabManager) read(ctx context.Context) (string, error) {
	out, err := m.Exec("crontab", []string{"-l"}, "")
	if err != nil {
		// Sin crontab instalado, crontab -l falla; se trata como vacío.
		return "", nil
	}
	return string(out), nil
}

func (m *CrontabManager) write(ctx context.Context, content string) error {
	_, err := m.Exec("crontab", []string{"-"}, content)
	return err
}

// systemCrontab ejecuta el binario crontab del sistema.
func systemCrontab(name string, args []string, stdin string) ([]byte, error) {
	return runCommand(name, args, stdin)
}
