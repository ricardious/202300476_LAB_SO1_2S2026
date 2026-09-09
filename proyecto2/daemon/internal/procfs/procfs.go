// Package procfs lee, deserializa y valida la instantánea JSON que expone el
// módulo del kernel en /proc/continfo_pr2_so1_202300476.
//
// El parser es "estricto/tolerante": rechaza documentos que no cumplan el
// contrato mínimo (versión, memoria y procesos bien formados) pero ignora
// campos desconocidos y tolera flotantes imprecisos y entradas truncadas sin
// romper la estructura tipada.
package procfs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// ExpectedSchemaVersion es la versión de contrato que el parser entiende.
const ExpectedSchemaVersion = 1

// ErrInvalidSnapshot señala que el documento no cumple el contrato mínimo.
var ErrInvalidSnapshot = errors.New("procfs: snapshot inválida")

// Memory resume el uso de memoria del sistema reportado por el módulo.
type Memory struct {
	TotalKB int64 `json:"total_kb"`
	FreeKB  int64 `json:"free_kb"`
	UsedKB  int64 `json:"used_kb"`
}

// Process describe un proceso (general o de contenedor) observado por el kernel.
type Process struct {
	PID           int64   `json:"pid"`
	Name          string  `json:"name"`
	Cmdline       string  `json:"cmdline"`
	ContainerID   string  `json:"container_id"`
	VSZKB         int64   `json:"vsz_kb"`
	RSSKB         int64   `json:"rss_kb"`
	MemoryPercent float64 `json:"memory_percent"`
	CPUPercent    float64 `json:"cpu_percent"`
}

// Snapshot es el documento raíz emitido por el módulo /proc.
type Snapshot struct {
	SchemaVersion   int       `json:"schema_version"`
	TimestampUnixNS int64     `json:"timestamp_unix_ns"`
	Memory          Memory    `json:"memory"`
	Processes       []Process `json:"processes"`
}

// Timestamp convierte el tiempo en nanosegundos a time.Time.
func (s Snapshot) Timestamp() time.Time {
	return time.Unix(0, s.TimestampUnixNS)
}

// Reader abstrae el origen de los bytes de la instantánea (archivo, fixture).
type Reader interface {
	Read() ([]byte, error)
}

// FileReader lee la instantánea desde un archivo del sistema.
type FileReader struct {
	Path string
}

// Read implementa Reader leyendo el archivo en Path.
func (r FileReader) Read() ([]byte, error) {
	return os.ReadFile(r.Path)
}

// Parse convierte bytes en una Snapshot validada.
func Parse(data []byte) (Snapshot, error) {
	return parseReader(bytes.NewReader(data))
}

func parseReader(r io.Reader) (Snapshot, error) {
	dec := json.NewDecoder(r)
	dec.UseNumber()

	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return Snapshot{}, fmt.Errorf("%w: decodificación: %v", ErrInvalidSnapshot, err)
	}

	// Decodificación estricta del contrato tipado.
	var snap Snapshot
	if err := decodeStrict(raw, &snap); err != nil {
		return Snapshot{}, err
	}

	if err := snap.Validate(); err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}

// decodeStrict deserializa los campos conocidos y descarta los desconocidos.
func decodeStrict(raw map[string]json.RawMessage, snap *Snapshot) error {
	// schema_version es obligatorio.
	if v, ok := raw["schema_version"]; ok {
		if err := json.Unmarshal(v, &snap.SchemaVersion); err != nil {
			return fmt.Errorf("%w: schema_version inválido: %v", ErrInvalidSnapshot, err)
		}
	} else {
		return fmt.Errorf("%w: falta schema_version", ErrInvalidSnapshot)
	}
	if snap.SchemaVersion != ExpectedSchemaVersion {
		return fmt.Errorf("%w: versión %d no soportada (esperada %d)", ErrInvalidSnapshot, snap.SchemaVersion, ExpectedSchemaVersion)
	}

	if v, ok := raw["timestamp_unix_ns"]; ok {
		_ = json.Unmarshal(v, &snap.TimestampUnixNS)
	}
	// memory es obligatorio según el contrato mínimo.
	v, ok := raw["memory"]
	if !ok {
		return fmt.Errorf("%w: falta memory", ErrInvalidSnapshot)
	}
	if err := json.Unmarshal(v, &snap.Memory); err != nil {
		return fmt.Errorf("%w: memory inválido: %v", ErrInvalidSnapshot, err)
	}
	// processes es obligatorio (puede estar vacío pero debe existir).
	v, ok = raw["processes"]
	if !ok {
		return fmt.Errorf("%w: falta processes", ErrInvalidSnapshot)
	}
	if err := json.Unmarshal(v, &snap.Processes); err != nil {
		return fmt.Errorf("%w: processes inválido: %v", ErrInvalidSnapshot, err)
	}
	return nil
}

// Validate comprueba las invariantes del contrato mínimo.
func (s Snapshot) Validate() error {
	if s.Memory.TotalKB < 0 {
		return fmt.Errorf("%w: total_kb negativo", ErrInvalidSnapshot)
	}
	for _, p := range s.Processes {
		if p.PID <= 0 {
			return fmt.Errorf("%w: proceso con PID inválido %d", ErrInvalidSnapshot, p.PID)
		}
		if p.VSZKB < 0 || p.RSSKB < 0 {
			return fmt.Errorf("%w: proceso %d con memoria negativa", ErrInvalidSnapshot, p.PID)
		}
	}
	return nil
}

// FindByPID localiza un proceso por su PID.
func (s Snapshot) FindByPID(pid int64) (Process, bool) {
	for _, p := range s.Processes {
		if p.PID == pid {
			return p, true
		}
	}
	return Process{}, false
}

// Containers devuelve los procesos que el módulo asoció a un contenedor.
func (s Snapshot) Containers() []Process {
	var out []Process
	for _, p := range s.Processes {
		if p.ContainerID != "" {
			out = append(out, p)
		}
	}
	return out
}

// IsTruncated detecta un documento cortado (JSON incompleto).
func IsTruncated(data []byte) bool {
	trimmed := strings.TrimSpace(string(data))
	return trimmed != "" && !json.Valid(data)
}
