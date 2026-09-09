// Package policy implementa la política determinista de selección y
// conservación de contenedores.
//
// Reglas del plan maestro:
//   - ordenar candidatos por RAM, VSZ, RSS y CPU con desempate determinista;
//   - conservar como mínimo 3 contenedores de bajo consumo y 2 de alto consumo;
//   - excluir siempre Grafana, Valkey y cualquier contenedor con etiqueta protegida.
package policy

import (
	"sort"

	"github.com/202300476/p2-daemon/internal/config"
	"github.com/202300476/p2-daemon/internal/docker"
)

// Candidate es un contenedor administrado con sus métricas enriquecidas.
type Candidate struct {
	Container  docker.Container
	MemPercent float64
	VSZKB      int64
	RSSKB      int64
	CPU        float64
	Tier       Tier
}

// Tier clasifica el consumo de un contenedor.
type Tier int

const (
	// TierLow indica consumo bajo.
	TierLow Tier = iota
	// TierHigh indica consumo alto.
	TierHigh
)

// Decision resume el resultado de aplicar la política en un ciclo.
type Decision struct {
	// Survivors son los contenedores que se conservan.
	Survivors []Candidate
	// RemovalCandidates son los contenedores excedentes, ordenados de mayor a
	// menor impacto (los primeros se eliminan primero).
	RemovalCandidates []Candidate
	// Excluded son los contenedores protegidos o sin métricas válidas.
	Excluded []Candidate
}

// Enricher completa un Candidate con métricas del kernel.
type Enricher interface {
	// Enrich devuelve las métricas de un contenedor a partir de su PID de host.
	Enrich(hostPID int) (mem, vsz, rss, cpu float64, ok bool)
}

// MetricFunc adapta una función simple al contrato Enricher.
type MetricFunc func(hostPID int) (mem, vsz, rss, cpu float64, ok bool)

// Enrich implementa Enricher delegando en la función.
func (f MetricFunc) Enrich(hostPID int) (float64, float64, float64, float64, bool) {
	return f(hostPID)
}

// Resolver decide si un contenedor está administrado o protegido.
type Resolver interface {
	IsManaged(labels map[string]string) bool
	IsProtected(name string, labels map[string]string) bool
}

// configResolver adapta la configuración al contrato Resolver.
type configResolver struct {
	cfg config.Config
}

// NewResolver construye un Resolver a partir de la configuración.
func NewResolver(cfg config.Config) Resolver {
	return configResolver{cfg: cfg}
}

// IsManaged implementa Resolver.IsManaged.
func (r configResolver) IsManaged(labels map[string]string) bool {
	return r.cfg.IsManaged(labels)
}

// IsProtected implementa Resolver.IsProtected.
func (r configResolver) IsProtected(name string, labels map[string]string) bool {
	return r.cfg.ProtectedName(name) || r.cfg.IsProtectedLabel(labels)
}

// Policy agrupa la configuración de conservación y clasificación.
type Policy struct {
	MinLow          int
	MinHigh         int
	HighThresholdKB int64
	Resolver        Resolver
}

// New construye una Policy desde la configuración.
func New(cfg config.Config) Policy {
	return Policy{
		MinLow:   cfg.MinLow,
		MinHigh:  cfg.MinHigh,
		Resolver: NewResolver(cfg),
	}
}

// Decide aplica la política sobre la lista de contenedores del proyecto y sus
// métricas. Devuelve supervivientes, candidatos a eliminar y excluidos.
func (p Policy) Decide(managed []docker.Container, env Enricher) Decision {
	dec := Decision{}

	var cands []Candidate
	for _, c := range managed {
		if p.Resolver.IsProtected(c.DisplayName(), c.Labels) {
			dec.Excluded = append(dec.Excluded, Candidate{Container: c})
			continue
		}
		if c.HostPID <= 0 {
			dec.Excluded = append(dec.Excluded, Candidate{Container: c})
			continue
		}
		mem, vsz, rss, cpu, ok := env.Enrich(c.HostPID)
		if !ok {
			dec.Excluded = append(dec.Excluded, Candidate{Container: c})
			continue
		}
		cands = append(cands, Candidate{
			Container:  c,
			MemPercent: mem,
			VSZKB:      int64(vsz),
			RSSKB:      int64(rss),
			CPU:        cpu,
		})
	}

	sort.Sort(byResource(cands))
	p.assignTiers(cands)

	if len(cands) <= p.MinLow+p.MinHigh {
		dec.Survivors = cands
		return dec
	}

	remove := map[string]bool{}
	if p.MinHigh > 0 {
		for i := p.MinHigh; i < len(cands); i++ {
			if cands[i].Tier == TierHigh {
				remove[cands[i].Container.ID] = true
			}
		}
	}
	if p.MinLow > 0 {
		for i := 0; i < len(cands)-p.MinLow; i++ {
			if cands[i].Tier == TierLow {
				remove[cands[i].Container.ID] = true
			}
		}
	}

	for _, c := range cands {
		if remove[c.Container.ID] {
			dec.RemovalCandidates = append(dec.RemovalCandidates, c)
		} else {
			dec.Survivors = append(dec.Survivors, c)
		}
	}
	return dec
}

// assignTiers clasifica cada candidato. Si hay umbral configurado se usa el
// valor de memoria; si no, se clasifica por posición en el ranking ordenado:
// los minHigh primeros son de alto consumo y los minLow últimos de bajo.
func (p Policy) assignTiers(cands []Candidate) {
	if p.HighThresholdKB > 0 {
		for i := range cands {
			if cands[i].Container.MemoryKB >= p.HighThresholdKB {
				cands[i].Tier = TierHigh
			} else {
				cands[i].Tier = TierLow
			}
		}
		return
	}
	for i := range cands {
		switch {
		case i < p.MinHigh:
			cands[i].Tier = TierHigh
		case i >= len(cands)-p.MinLow:
			cands[i].Tier = TierLow
		default:
			// Zona intermedia: se considera de bajo consumo para poder
			// eliminarla sin violar la cuota mínima de altos.
			cands[i].Tier = TierLow
		}
	}
}

// byResource ordena de forma determinista por RAM, VSZ, RSS y CPU, con
// desempate final por nombre e ID.
type byResource []Candidate

func (b byResource) Len() int      { return len(b) }
func (b byResource) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byResource) Less(i, j int) bool {
	a, c := b[i], b[j]
	if a.MemPercent != c.MemPercent {
		return a.MemPercent > c.MemPercent
	}
	if a.VSZKB != c.VSZKB {
		return a.VSZKB > c.VSZKB
	}
	if a.RSSKB != c.RSSKB {
		return a.RSSKB > c.RSSKB
	}
	if a.CPU != c.CPU {
		return a.CPU > c.CPU
	}
	an, cn := a.Container.DisplayName(), c.Container.DisplayName()
	if an != cn {
		return an < cn
	}
	return a.Container.ID < c.Container.ID
}
