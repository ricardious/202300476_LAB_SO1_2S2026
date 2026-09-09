package config

import (
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.ProcPath != DefaultProcPath {
		t.Errorf("ProcPath = %q", cfg.ProcPath)
	}
	if cfg.LoopInterval != 20*time.Second {
		t.Errorf("LoopInterval = %v", cfg.LoopInterval)
	}
	if cfg.MinLow != 3 || cfg.MinHigh != 2 {
		t.Errorf("cuotas = low %d high %d, esperaba 3/2", cfg.MinLow, cfg.MinHigh)
	}
	if cfg.ProtectedName("grafana") != true || cfg.ProtectedName("VALKEY") != true {
		t.Error("grafana/valkey deben estar protegidos por nombre")
	}
	if cfg.ProtectedName("so1-p2-grafana") != true || cfg.ProtectedName("SO1-P2-VALKEY") != true {
		t.Error("los nombres reales de Compose so1-p2-* deben estar protegidos por nombre")
	}
	if cfg.EBPFMode != "auto" {
		t.Errorf("EBPFMode = %q, esperaba auto", cfg.EBPFMode)
	}
	if cfg.ModuleLoadScript != "scripts/load-module.sh" || cfg.ModuleUnloadScript != "scripts/unload-module.sh" {
		t.Errorf("scripts de módulo = %q/%q", cfg.ModuleLoadScript, cfg.ModuleUnloadScript)
	}
	if cfg.CronCommand != "/usr/local/bin/so1-p2-generate-containers.sh" || cfg.CronSchedule != "*/2 * * * *" {
		t.Errorf("cron = %q @ %q", cfg.CronCommand, cfg.CronSchedule)
	}
	if cfg.ComposeFile != "deploy/docker-compose.yml" {
		t.Errorf("ComposeFile = %q", cfg.ComposeFile)
	}
}

func TestValidateRange(t *testing.T) {
	ok := Default()
	if err := ok.Validate(); err != nil {
		t.Fatalf("configuración default debe ser válida: %v", err)
	}

	tooShort := Default()
	tooShort.LoopInterval = 5 * time.Second
	if err := tooShort.Validate(); err == nil {
		t.Error("loop < 20s debe ser rechazado")
	}

	tooLong := Default()
	tooLong.LoopInterval = 120 * time.Second
	if err := tooLong.Validate(); err == nil {
		t.Error("loop > 60s debe ser rechazado")
	}
}

func TestProtectedLabel(t *testing.T) {
	cfg := Default()
	labels := map[string]string{"so1.p2.protect": "1", "app": "grafana"}
	if !cfg.IsProtectedLabel(labels) {
		t.Error("etiqueta protegida debe detectarse")
	}
	if cfg.IsProtectedLabel(map[string]string{"app": "nginx"}) {
		t.Error("etiqueta no protegida no debe detectarse")
	}
}

func TestIsManaged(t *testing.T) {
	cfg := Default()
	if !cfg.IsManaged(map[string]string{"so1.p2.managed": "1"}) {
		t.Error("contenedor con etiqueta administrada debe reconocerse")
	}
	if cfg.IsManaged(map[string]string{}) {
		t.Error("contenedor sin etiqueta no debe considerarse administrado")
	}
}

func TestFromEnvOverrides(t *testing.T) {
	t.Setenv("SO1_P2_LOOP_INTERVAL", "30s")
	t.Setenv("SO1_P2_MIN_LOW", "4")
	t.Setenv("SO1_P2_MIN_HIGH", "1")
	t.Setenv("SO1_P2_PROTECTED_NAMES", "grafana,valkey,admin")
	t.Setenv("SO1_P2_VALKEY_ADDR", "localhost:6380")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if cfg.LoopInterval != 30*time.Second {
		t.Errorf("LoopInterval = %v", cfg.LoopInterval)
	}
	if cfg.MinLow != 4 || cfg.MinHigh != 1 {
		t.Errorf("cuotas = %d/%d", cfg.MinLow, cfg.MinHigh)
	}
	if !cfg.ProtectedName("admin") {
		t.Error("admin debe protegerse tras override")
	}
	if cfg.ValkeyAddr != "localhost:6380" {
		t.Errorf("ValkeyAddr = %q", cfg.ValkeyAddr)
	}
}

func TestFromEnvInvalidDuration(t *testing.T) {
	t.Setenv("SO1_P2_LOOP_INTERVAL", "abc")
	if _, err := FromEnv(); err == nil {
		t.Error("duración inválida debe producir error")
	}
}

func TestFromEnvEBPFAndModule(t *testing.T) {
	t.Setenv("SO1_P2_EBPF_MODE", "REQUIRED")
	t.Setenv("SO1_P2_MODULE_LOAD_SCRIPT", "/opt/so1-p2/scripts/load-module.sh")
	t.Setenv("SO1_P2_MODULE_UNLOAD_SCRIPT", "/opt/so1-p2/scripts/unload-module.sh")
	t.Setenv("SO1_P2_CRON_COMMAND", "/opt/so1-p2/bin/generate.sh")
	t.Setenv("SO1_P2_CRON_SCHEDULE", "*/5 * * * *")
	t.Setenv("SO1_P2_COMPOSE_FILE", "/opt/so1-p2/deploy/docker-compose.yml")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if cfg.EBPFMode != "required" {
		t.Errorf("EBPFMode = %q, esperaba required (normalizado)", cfg.EBPFMode)
	}
	if cfg.ModuleLoadScript != "/opt/so1-p2/scripts/load-module.sh" {
		t.Errorf("ModuleLoadScript = %q", cfg.ModuleLoadScript)
	}
	if cfg.CronCommand != "/opt/so1-p2/bin/generate.sh" {
		t.Errorf("CronCommand = %q", cfg.CronCommand)
	}
	if cfg.CronSchedule != "*/5 * * * *" {
		t.Errorf("CronSchedule = %q", cfg.CronSchedule)
	}
	if cfg.ComposeFile != "/opt/so1-p2/deploy/docker-compose.yml" {
		t.Errorf("ComposeFile = %q", cfg.ComposeFile)
	}
}

func TestValidateEBPFMode(t *testing.T) {
	bad := Default()
	bad.EBPFMode = "sometimes"
	if err := bad.Validate(); err == nil {
		t.Error("EBPFMode inválido debe ser rechazado")
	}
	for _, m := range []string{"auto", "required", "off"} {
		ok := Default()
		ok.EBPFMode = m
		if err := ok.Validate(); err != nil {
			t.Errorf("EBPFMode=%s debe ser válido: %v", m, err)
		}
	}
}

func TestValidateModuleScripts(t *testing.T) {
	cfg := Default()
	cfg.LoadModuleOnStart = true
	cfg.ModuleLoadScript = ""
	if err := cfg.Validate(); err == nil {
		t.Error("LoadModuleOnStart sin script debe ser rechazado")
	}
	cfg = Default()
	cfg.UnloadModuleOnStop = true
	cfg.ModuleUnloadScript = ""
	if err := cfg.Validate(); err == nil {
		t.Error("UnloadModuleOnStop sin script debe ser rechazado")
	}
}

func TestFromEnvComposeFileVacioDesactiva(t *testing.T) {
	t.Setenv("SO1_P2_COMPOSE_FILE", "")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if cfg.ComposeFile != "" {
		t.Errorf("ComposeFile = %q, esperaba vacío (desactiva el auto-arranque)", cfg.ComposeFile)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("ComposeFile vacío debe ser válido: %v", err)
	}
}

func TestCSV(t *testing.T) {
	got := splitCSV(" a , b,c ,")
	if len(got) != 3 {
		t.Fatalf("splitCSV = %v, esperaba 3 elementos", got)
	}
}
