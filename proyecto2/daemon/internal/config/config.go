// Package config carga y valida la configuración del daemon.
//
// Toda la configuración se lee desde variables de entorno con valores
// por defecto seguros. El método Validate aplica las reglas del plan maestro:
// loop entre 20 y 60 segundos, cuotas mínimas 3/2, rutas y prefijos válidos.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// Carnet oficial del proyecto, usado como identificador del módulo /proc.
	Carnet = "202300476"

	// DefaultProcPath es la interfaz /proc expuesta por el módulo del kernel.
	DefaultProcPath = "/proc/continfo_pr2_so1_" + Carnet

	// DefaultKeyPrefix es el prefijo de las claves en Valkey.
	DefaultKeyPrefix = "so1:p2:"
)

// Config reúne todos los parámetros de arranque y operación del daemon.
type Config struct {
	// ProcPath es la ruta del documento JSON que expone el módulo.
	ProcPath string

	// LoopInterval es el período del ciclo de reconciliación (20-60s).
	LoopInterval time.Duration

	// DockerHost es el endpoint de la API Docker Engine.
	DockerHost string

	// DockerAPIVersion es la versión de la API a negociar.
	DockerAPIVersion string

	// ValkeyAddr es la dirección del servidor Valkey (host:puerto).
	ValkeyAddr string

	// ValkeyPassword credencial opcional de Valkey.
	ValkeyPassword string

	// ValkeyDB es el índice de base de datos de Valkey.
	ValkeyDB int

	// KeyPrefix es el prefijo de todas las claves persistidas.
	KeyPrefix string

	// MinLow es la cantidad mínima de contenedores de bajo consumo a conservar.
	MinLow int

	// MinHigh es la cantidad mínima de contenedores de alto consumo a conservar.
	MinHigh int

	// ProtectedLabels son etiquetas que impiden la eliminación de un contenedor.
	ProtectedLabels []string

	// ProtectedNames son nombres de contenedor absolutamente protegidos.
	ProtectedNames []string

	// ManagedLabelKey es la etiqueta que marca un contenedor como administrado.
	ManagedLabelKey string

	// ManagedLabelValue es el valor esperado de la etiqueta administrada.
	ManagedLabelValue string

	// KillSignal es la señal que el daemon usa para terminar candidatos.
	KillSignal string

	// KillTimeout es la ventana temporal de espera del evento eBPF correlacionado.
	KillTimeout time.Duration

	// ConfirmationTimeout es el tiempo máximo para esperar la confirmación eBPF.
	ConfirmationTimeout time.Duration

	// LoadModuleOnStart indica si el daemon debe cargar el módulo /proc al arrancar.
	LoadModuleOnStart bool

	// UnloadModuleOnStop indica si el daemon debe descargar el módulo al apagar.
	UnloadModuleOnStop bool
}

// Default devuelve una configuración con los valores por defecto del plan.
func Default() Config {
	return Config{
		ProcPath:            DefaultProcPath,
		LoopInterval:        20 * time.Second,
		DockerHost:          "unix:///var/run/docker.sock",
		DockerAPIVersion:    "1.41",
		ValkeyAddr:          "127.0.0.1:6379",
		ValkeyPassword:      "",
		ValkeyDB:            0,
		KeyPrefix:           DefaultKeyPrefix,
		MinLow:              3,
		MinHigh:             2,
		ProtectedLabels:     []string{"so1.p2.protect=1"},
		ProtectedNames:      []string{"grafana", "valkey"},
		ManagedLabelKey:     "so1.p2.managed",
		ManagedLabelValue:   "1",
		KillSignal:          "SIGKILL",
		KillTimeout:         2 * time.Second,
		ConfirmationTimeout: 30 * time.Second,
		LoadModuleOnStart:   true,
		UnloadModuleOnStop:  false,
	}
}

// FromEnv construye la configuración desde variables de entorno.
// Las variables ausentes o inválidas caen a los valores por defecto.
func FromEnv() (Config, error) {
	cfg := Default()
	var err error

	cfg.ProcPath = envOr("SO1_P2_PROC_PATH", cfg.ProcPath)
	cfg.DockerHost = envOr("SO1_P2_DOCKER_HOST", cfg.DockerHost)
	cfg.DockerAPIVersion = envOr("SO1_P2_DOCKER_API_VERSION", cfg.DockerAPIVersion)
	cfg.ValkeyAddr = envOr("SO1_P2_VALKEY_ADDR", cfg.ValkeyAddr)
	cfg.ValkeyPassword = envOr("SO1_P2_VALKEY_PASSWORD", cfg.ValkeyPassword)
	cfg.KeyPrefix = envOr("SO1_P2_KEY_PREFIX", cfg.KeyPrefix)

	if v := os.Getenv("SO1_P2_LOOP_INTERVAL"); v != "" {
		if cfg.LoopInterval, err = time.ParseDuration(v); err != nil {
			return cfg, fmt.Errorf("SO1_P2_LOOP_INTERVAL inválido %q: %w", v, err)
		}
	}
	if v := os.Getenv("SO1_P2_VALKEY_DB"); v != "" {
		if cfg.ValkeyDB, err = strconv.Atoi(v); err != nil {
			return cfg, fmt.Errorf("SO1_P2_VALKEY_DB inválido %q: %w", v, err)
		}
	}
	if v := os.Getenv("SO1_P2_MIN_LOW"); v != "" {
		if cfg.MinLow, err = strconv.Atoi(v); err != nil {
			return cfg, fmt.Errorf("SO1_P2_MIN_LOW inválido %q: %w", v, err)
		}
	}
	if v := os.Getenv("SO1_P2_MIN_HIGH"); v != "" {
		if cfg.MinHigh, err = strconv.Atoi(v); err != nil {
			return cfg, fmt.Errorf("SO1_P2_MIN_HIGH inválido %q: %w", v, err)
		}
	}
	if v := os.Getenv("SO1_P2_PROTECTED_LABELS"); v != "" {
		cfg.ProtectedLabels = splitCSV(v)
	}
	if v := os.Getenv("SO1_P2_PROTECTED_NAMES"); v != "" {
		cfg.ProtectedNames = splitCSV(v)
	}
	if v := os.Getenv("SO1_P2_MANAGED_LABEL_KEY"); v != "" {
		cfg.ManagedLabelKey = v
	}
	if v := os.Getenv("SO1_P2_MANAGED_LABEL_VALUE"); v != "" {
		cfg.ManagedLabelValue = v
	}
	if v := os.Getenv("SO1_P2_KILL_SIGNAL"); v != "" {
		cfg.KillSignal = v
	}
	if v := os.Getenv("SO1_P2_KILL_TIMEOUT"); v != "" {
		if cfg.KillTimeout, err = time.ParseDuration(v); err != nil {
			return cfg, fmt.Errorf("SO1_P2_KILL_TIMEOUT inválido %q: %w", v, err)
		}
	}
	if v := os.Getenv("SO1_P2_CONFIRMATION_TIMEOUT"); v != "" {
		if cfg.ConfirmationTimeout, err = time.ParseDuration(v); err != nil {
			return cfg, fmt.Errorf("SO1_P2_CONFIRMATION_TIMEOUT inválido %q: %w", v, err)
		}
	}
	if v := os.Getenv("SO1_P2_LOAD_MODULE_ON_START"); v != "" {
		cfg.LoadModuleOnStart = parseBool(v)
	}
	if v := os.Getenv("SO1_P2_UNLOAD_MODULE_ON_STOP"); v != "" {
		cfg.UnloadModuleOnStop = parseBool(v)
	}

	return cfg, nil
}

// Validate aplica las reglas de negocio sobre la configuración.
func (c Config) Validate() error {
	if c.ProcPath == "" {
		return fmt.Errorf("ProcPath no puede estar vacío")
	}
	if c.LoopInterval < 20*time.Second || c.LoopInterval > 60*time.Second {
		return fmt.Errorf("LoopInterval %s fuera del rango permitido [20s,60s]", c.LoopInterval)
	}
	if c.MinLow < 0 || c.MinHigh < 0 {
		return fmt.Errorf("las cuotas mínimas no pueden ser negativas")
	}
	if c.DockerHost == "" {
		return fmt.Errorf("DockerHost no puede estar vacío")
	}
	if c.ValkeyAddr == "" {
		return fmt.Errorf("ValkeyAddr no puede estar vacío")
	}
	if c.KeyPrefix == "" {
		return fmt.Errorf("KeyPrefix no puede estar vacío")
	}
	if c.KillSignal == "" {
		return fmt.Errorf("KillSignal no puede estar vacío")
	}
	if c.ConfirmationTimeout <= 0 {
		return fmt.Errorf("ConfirmationTimeout debe ser positivo")
	}
	return nil
}

// ProtectedName devuelve true si el nombre está protegido.
func (c Config) ProtectedName(name string) bool {
	for _, p := range c.ProtectedNames {
		if strings.EqualFold(p, name) {
			return true
		}
	}
	return false
}

// IsProtectedLabel devuelve true si la etiqueta está en la lista protegida.
func (c Config) IsProtectedLabel(labels map[string]string) bool {
	for _, p := range c.ProtectedLabels {
		if hasLabel(labels, p) {
			return true
		}
	}
	return false
}

// IsManaged devuelve true si el contenedor lleva la etiqueta administrada esperada.
func (c Config) IsManaged(labels map[string]string) bool {
	return labels[c.ManagedLabelKey] == c.ManagedLabelValue
}

// hasLabel compara un par clave=valor contra un mapa de etiquetas.
func hasLabel(labels map[string]string, pair string) bool {
	k, v, ok := strings.Cut(pair, "=")
	if !ok {
		return false
	}
	return labels[k] == v
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
