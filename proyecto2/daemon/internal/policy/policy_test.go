package policy

import (
	"testing"

	"github.com/202300476/p2-daemon/internal/config"
	"github.com/202300476/p2-daemon/internal/docker"
)

// envFromMap construye un Enricher que devuelve métricas por PID de host.
func envFromMap(m map[int][]float64) Enricher {
	return MetricFunc(func(pid int) (float64, float64, float64, float64, bool) {
		v, ok := m[pid]
		if !ok {
			return 0, 0, 0, 0, false
		}
		return v[0], v[1], v[2], v[3], true
	})
}

func newContainer(id, name string, pid int) docker.Container {
	return docker.Container{ID: id, Name: "/" + name, HostPID: pid}
}

func TestDecidePreserves3Low2HighPositional(t *testing.T) {
	cfg := config.Default()
	pol := New(cfg)

	// 10 contenedores con consumo descendente de memoria.
	var managed []docker.Container
	env := map[int][]float64{}
	for i := 0; i < 10; i++ {
		pid := 1000 + i
		id := string(rune('a' + i))
		managed = append(managed, newContainer(id, "c"+string(rune('a'+i)), pid))
		// memoria decreciente: 100, 90, ... 10
		env[pid] = []float64{float64(100 - i*10), float64((100 - i*10) * 100), float64((100 - i*10) * 10), 1.0}
	}

	dec := pol.Decide(managed, envFromMap(env))

	if len(dec.Survivors) != 5 {
		t.Fatalf("supervivientes = %d, esperaba 5", len(dec.Survivors))
	}
	if len(dec.RemovalCandidates) != 5 {
		t.Fatalf("candidatos = %d, esperaba 5", len(dec.RemovalCandidates))
	}

	// El candidato más consumidor debe ser el primero en eliminarse.
	if dec.RemovalCandidates[0].Container.Name != "/cc" {
		t.Errorf("primer candidato = %s, esperaba /cc (consumo 80)", dec.RemovalCandidates[0].Container.Name)
	}
	// El de mayor consumo total debe conservarse (top 2 altos).
	if dec.Survivors[0].Container.Name != "/ca" {
		t.Errorf("primer superviviente = %s, esperaba /ca", dec.Survivors[0].Container.Name)
	}
}

func TestDecideTieBreakDeterministic(t *testing.T) {
	cfg := config.Default()
	pol := New(cfg)

	// Dos contenedores con métricas idénticas: el desempate es por nombre.
	managed := []docker.Container{
		newContainer("zz", "zeta", 1),
		newContainer("aa", "alpha", 2),
	}
	env := map[int][]float64{
		1: {50, 100, 50, 10},
		2: {50, 100, 50, 10},
	}
	dec := pol.Decide(managed, envFromMap(env))
	if len(dec.Survivors) != 2 {
		t.Fatalf("ambos deben sobrevivir (dentro del mínimo), obtuvimos %d", len(dec.Survivors))
	}
	// Orden determinista: alpha (aa) antes que zeta (zz).
	if dec.Survivors[0].Container.Name != "/alpha" {
		t.Errorf("primer superviviente = %s, esperaba /alpha", dec.Survivors[0].Container.Name)
	}
}

func TestDecideProtectsGrafanaAndValkey(t *testing.T) {
	cfg := config.Default()
	pol := New(cfg)

	managed := []docker.Container{
		newContainer("id-graf", "grafana", 5000),
		newContainer("id-valk", "valkey", 5001),
		newContainer("id-worker", "worker", 5002),
	}
	env := map[int][]float64{
		5000: {90, 900, 900, 5},
		5001: {80, 800, 800, 5},
		5002: {70, 700, 700, 5},
	}
	dec := pol.Decide(managed, envFromMap(env))
	if len(dec.Excluded) != 2 {
		t.Fatalf("excluidos = %d, esperaba 2 (grafana y valkey)", len(dec.Excluded))
	}
	for _, c := range dec.Excluded {
		if c.Container.DisplayName() != "grafana" && c.Container.DisplayName() != "valkey" {
			t.Errorf("excluido inesperado: %s", c.Container.DisplayName())
		}
	}
	if len(dec.RemovalCandidates) != 0 {
		t.Error("grafana/valkey nunca deben ser candidatos a eliminación")
	}
}

func TestDecideProtectsByLabel(t *testing.T) {
	cfg := config.Default()
	pol := New(cfg)

	prot := docker.Container{ID: "prot", Name: "/db", HostPID: 1, Labels: map[string]string{"so1.p2.protect": "1"}}
	worker := newContainer("wk", "worker", 2)
	env := map[int][]float64{1: {90, 900, 900, 5}, 2: {10, 100, 100, 1}}

	dec := pol.Decide([]docker.Container{prot, worker}, envFromMap(env))
	if len(dec.Excluded) != 1 || dec.Excluded[0].Container.ID != "prot" {
		t.Errorf("contenedor etiquetado como protegido debe excluirse, excluidos=%+v", dec.Excluded)
	}
}

func TestDecideSkipsNoMetrics(t *testing.T) {
	cfg := config.Default()
	pol := New(cfg)

	noMetrics := newContainer("nm", "no-metrics", 999) // sin entrada en env
	worker := newContainer("wk", "worker", 2)
	env := map[int][]float64{2: {10, 100, 100, 1}}

	dec := pol.Decide([]docker.Container{noMetrics, worker}, envFromMap(env))
	if len(dec.Excluded) != 1 || dec.Excluded[0].Container.ID != "nm" {
		t.Errorf("contenedor sin métricas debe excluirse, excluidos=%+v", dec.Excluded)
	}
}

func TestDecideSkipsNoHostPID(t *testing.T) {
	cfg := config.Default()
	pol := New(cfg)

	noPID := newContainer("np", "no-pid", 0) // HostPID 0 = sin proceso
	dec := pol.Decide([]docker.Container{noPID}, envFromMap(map[int][]float64{}))
	if len(dec.Excluded) != 1 {
		t.Fatalf("contenedor sin PID debe excluirse, excluidos=%+v", dec.Excluded)
	}
}

func TestDecideWithinQuotaSurvivesAll(t *testing.T) {
	cfg := config.Default()
	pol := New(cfg)

	// 4 contenedores: por debajo del umbral 3+2=5, todos sobreviven.
	var managed []docker.Container
	env := map[int][]float64{}
	for i := 0; i < 4; i++ {
		pid := 100 + i
		managed = append(managed, newContainer(string(rune('a'+i)), "c", pid))
		env[pid] = []float64{float64(i + 1), 100, 50, 1}
	}
	dec := pol.Decide(managed, envFromMap(env))
	if len(dec.Survivors) != 4 {
		t.Fatalf("supervivientes = %d, esperaba 4 (bajo el mínimo)", len(dec.Survivors))
	}
	if len(dec.RemovalCandidates) != 0 {
		t.Error("bajo el mínimo no debe haber candidatos")
	}
}

func TestNewWithThreshold(t *testing.T) {
	cfg := config.Default()
	pol := New(cfg)
	pol.HighThresholdKB = 5000

	high := newContainer("h", "high", 1)
	low := newContainer("l", "low", 2)
	high.MemoryKB = 10000
	low.MemoryKB = 100
	env := map[int][]float64{1: {50, 100, 50, 1}, 2: {5, 100, 50, 1}}

	dec := pol.Decide([]docker.Container{high, low}, envFromMap(env))
	if len(dec.Survivors) != 2 {
		t.Fatalf("supervivientes = %d", len(dec.Survivors))
	}
	foundHigh, foundLow := false, false
	for _, c := range dec.Survivors {
		if c.Tier == TierHigh {
			foundHigh = true
		}
		if c.Tier == TierLow {
			foundLow = true
		}
	}
	if !foundHigh || !foundLow {
		t.Error("esperaba un tier high y un tier low")
	}
}
