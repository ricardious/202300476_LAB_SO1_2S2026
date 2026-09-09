package cron

import (
	"bytes"
	"os/exec"
	"strings"
)

// runCommand ejecuta un comando pasándole entrada estándar y devolviendo la
// salida. Se usa para delegar en el binario crontab del sistema.
func runCommand(name string, args []string, stdin string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	if strings.TrimSpace(stdin) != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return nil, &execError{name: name, err: err, stderr: errBuf.String()}
	}
	return out.Bytes(), nil
}

// execError envuelve un error de ejecución con su stderr.
type execError struct {
	name   string
	err    error
	stderr string
}

func (e *execError) Error() string {
	if e.stderr != "" {
		return "cron: " + e.name + ": " + e.stderr
	}
	return "cron: " + e.name + ": " + e.err.Error()
}
