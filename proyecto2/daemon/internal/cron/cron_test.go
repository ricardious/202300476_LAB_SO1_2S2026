package cron

import (
	"context"
	"strings"
	"testing"
)

// fakeCrontab mantiene un crontab en memoria y registra las llamadas.
type fakeCrontab struct {
	content string
	readErr bool
	writeOK bool
	install string
	remove  bool
}

func (f *fakeCrontab) exec(name string, args []string, stdin string) ([]byte, error) {
	if name != "crontab" {
		return nil, nil
	}
	if len(args) == 1 && args[0] == "-l" {
		if f.readErr {
			return nil, ErrExec
		}
		return []byte(f.content), nil
	}
	if len(args) == 1 && args[0] == "-" {
		if !f.writeOK {
			return nil, ErrExec
		}
		f.content = stdin
		return nil, nil
	}
	return nil, nil
}

func newManager(f *fakeCrontab) *CrontabManager {
	return &CrontabManager{Exec: f.exec}
}

func TestInstallAndIsInstalled(t *testing.T) {
	f := &fakeCrontab{writeOK: true}
	m := newManager(f)
	ctx := context.Background()

	entry := Entry{Schedule: "*/2 * * * *", Command: "/x.sh"}
	if err := m.Install(ctx, entry); err != nil {
		t.Fatalf("install: %v", err)
	}
	if !strings.Contains(f.content, BeginMarker) {
		t.Error("el cronjob debe quedar delimitado por marcadores")
	}
	if !strings.Contains(f.content, "*/2 * * * * /x.sh") {
		t.Errorf("contenido no esperado:\n%s", f.content)
	}

	ok, err := m.IsInstalled(ctx)
	if err != nil || !ok {
		t.Fatalf("IsInstalled = %v, %v", ok, err)
	}

	// Instalar de nuevo no debe duplicar.
	if err := m.Install(ctx, entry); err != nil {
		t.Fatalf("install repetido: %v", err)
	}
	if strings.Count(f.content, BeginMarker) != 1 {
		t.Errorf("instalación no idempotente:\n%s", f.content)
	}
}

func TestRemoveOnlyProjectBlock(t *testing.T) {
	f := &fakeCrontab{writeOK: true, content: "# ajenos\n* * * * * /other.sh\n# BEGIN so1-p2-202300476\n*/2 * * * * /x.sh\n# END so1-p2-202300476\n"}
	m := newManager(f)

	if err := m.Remove(context.Background()); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if strings.Contains(f.content, BeginMarker) {
		t.Error("el bloque del proyecto debe retirarse")
	}
	if !strings.Contains(f.content, "/other.sh") {
		t.Error("las entradas ajenas deben conservarse")
	}
}

func TestRemoveWhenNotInstalled(t *testing.T) {
	f := &fakeCrontab{writeOK: true, content: "* * * * * /other.sh\n"}
	m := newManager(f)
	if err := m.Remove(context.Background()); err != nil {
		t.Fatalf("remove sin instalación: %v", err)
	}
	if f.content != "* * * * * /other.sh\n" {
		t.Errorf("contenido alterado:\n%q", f.content)
	}
}

func TestInstallReadErrorTreatedAsEmpty(t *testing.T) {
	f := &fakeCrontab{readErr: true, writeOK: true}
	m := newManager(f)
	if err := m.Install(context.Background(), Entry{Schedule: "*/2 * * * *", Command: "/x.sh"}); err != nil {
		t.Fatalf("install: %v", err)
	}
	if !strings.Contains(f.content, BeginMarker) {
		t.Error("sin crontab previo debe instalarse igual")
	}
}
