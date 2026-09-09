package procfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "..", "tests", "fixtures", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se pudo leer fixture %s: %v", name, err)
	}
	return data
}

func TestParseValidSnapshot(t *testing.T) {
	snap, err := Parse(fixture(t, "valid_snapshot.json"))
	if err != nil {
		t.Fatalf("esperaba parseo correcto, obtuve error: %v", err)
	}
	if snap.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, esperaba 1", snap.SchemaVersion)
	}
	if snap.Memory.TotalKB != 8388608 {
		t.Errorf("total_kb = %d, esperaba 8388608", snap.Memory.TotalKB)
	}
	if len(snap.Processes) != 5 {
		t.Fatalf("procesos = %d, esperaba 5", len(snap.Processes))
	}
	// Verificar enriquecimiento por PID.
	p, ok := snap.FindByPID(120)
	if !ok {
		t.Fatal("no se encontró el PID 120")
	}
	if p.Name != "memory-high" {
		t.Errorf("name = %q, esperaba memory-high", p.Name)
	}
	if _, ok := snap.FindByPID(99999); ok {
		t.Error("encontró un PID inexistente")
	}
}

func TestParseTolerantUnknownFields(t *testing.T) {
	snap, err := Parse(fixture(t, "unknown_fields.json"))
	if err != nil {
		t.Fatalf("el parser debe tolerar campos desconocidos: %v", err)
	}
	if len(snap.Processes) != 1 {
		t.Fatalf("procesos = %d, esperaba 1", len(snap.Processes))
	}
	if snap.Processes[0].Name != "memory-high" {
		t.Errorf("name = %q", snap.Processes[0].Name)
	}
}

func TestParseRejectsWrongSchema(t *testing.T) {
	_, err := Parse(fixture(t, "wrong_schema.json"))
	if err == nil {
		t.Fatal("esperaba error por versión de esquema no soportada")
	}
	if !strings.Contains(err.Error(), "no soportada") {
		t.Errorf("error inesperado: %v", err)
	}
}

func TestParseRejectsTruncated(t *testing.T) {
	data := fixture(t, "truncated_snapshot.json")
	if !IsTruncated(data) {
		t.Error("IsTruncated debe detectar el documento cortado")
	}
	if _, err := Parse(data); err == nil {
		t.Fatal("esperaba error de parseo para entrada truncada")
	}
}

func TestParseRejectsInvalid(t *testing.T) {
	cases := []string{
		``,
		`not json at all`,
		`{"schema_version": 1}`,
		`{"schema_version": 1, "processes": [{"pid": 0}]}`,
		`{"schema_version": 1, "processes": [{"pid": -5}]}`,
		`{"schema_version": 1, "memory": {"total_kb": -1}}`,
	}
	for _, c := range cases {
		if _, err := Parse([]byte(c)); err == nil {
			t.Errorf("esperaba error para input %q", c)
		}
	}
}

func TestParseRejectsMissingSections(t *testing.T) {
	base := `{"schema_version": 1, "memory": {"total_kb": 1}, "processes": []}`
	if _, err := Parse([]byte(base)); err != nil {
		t.Fatalf("contrato mínimo válido no debe fallar: %v", err)
	}

	if _, err := Parse([]byte(`{"schema_version": 1, "processes": []}`)); err == nil {
		t.Error("falta memory y debe producir error")
	}
	if _, err := Parse([]byte(`{"schema_version": 1, "memory": {"total_kb": 1}}`)); err == nil {
		t.Error("falta processes y debe producir error")
	}
}

func TestValidateInvariants(t *testing.T) {
	snap := Snapshot{
		SchemaVersion: 1,
		Processes: []Process{
			{PID: 1, VSZKB: 10, RSSKB: 5},
		},
	}
	if err := snap.Validate(); err != nil {
		t.Fatalf("snapshot válida no debe fallar: %v", err)
	}

	bad := Snapshot{SchemaVersion: 1, Processes: []Process{{PID: 1, VSZKB: -1, RSSKB: 5}}}
	if err := bad.Validate(); err == nil {
		t.Error("esperaba error por VSZ negativo")
	}
}

func TestContainersFilter(t *testing.T) {
	snap := Snapshot{
		Processes: []Process{
			{PID: 1, ContainerID: "c1"},
			{PID: 2, ContainerID: ""},
			{PID: 3, ContainerID: "c3"},
		},
	}
	cons := snap.Containers()
	if len(cons) != 2 {
		t.Fatalf("containers = %d, esperaba 2", len(cons))
	}
}

func TestTimestampConversion(t *testing.T) {
	snap := Snapshot{TimestampUnixNS: 1757100000000000000}
	if snap.Timestamp().IsZero() {
		t.Error("timestamp debe convertirse a time.Time")
	}
}
