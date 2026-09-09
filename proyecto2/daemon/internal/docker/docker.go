// Package docker implementa un cliente ligero de la API Docker Engine.
//
// Se conecta al socket Unix de Docker (o a un endpoint HTTP configurable) y
// ofrece tres operaciones: descubrir contenedores administrados por etiqueta,
// enriquecer un contenedor con su PID de host y métricas, y solicitar la
// terminación de un candidato. La abstracción ContainerAPI permite inyectar
// un mock (httptest) en las pruebas y enriquecer después el consumidor eBPF.
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// Container describe un contenedor administrado por el daemon.
type Container struct {
	ID       string            `json:"Id"`
	Names    []string          `json:"Names"`
	Image    string            `json:"Image"`
	State    string            `json:"State"`
	Labels   map[string]string `json:"Labels"`
	HostPID  int               `json:"HostPID"`
	Name     string            `json:"Name"`
	MemoryKB int64             `json:"MemoryKB"`
	CPU      float64           `json:"CPU"`
}

// DisplayName devuelve un nombre estable para el contenedor.
func (c Container) DisplayName() string {
	if c.Name != "" {
		return strings.TrimPrefix(c.Name, "/")
	}
	if len(c.Names) > 0 {
		return strings.TrimPrefix(c.Names[0], "/")
	}
	return c.ID
}

// ContainerAPI es el contrato del cliente Docker que usa el daemon.
type ContainerAPI interface {
	// ListManaged devuelve los contenedores en ejecución marcados con la
	// etiqueta administrada del proyecto.
	ListManaged(ctx context.Context, labelKey, labelValue string) ([]Container, error)

	// Inspect enriquece un contenedor con su PID de host (State.Pid) y
	// devuelve los metadatos completos.
	Inspect(ctx context.Context, id string) (Container, error)

	// Kill solicita la terminación de un contenedor con una señal dada.
	Kill(ctx context.Context, id, signal string) error

	// Ping verifica la disponibilidad del daemon Docker.
	Ping(ctx context.Context) error
}

// Client es una implementación concreta de ContainerAPI sobre la API Docker.
type Client struct {
	http *http.Client
	base string
}

// New construye un Client para el host indicado (ej. "unix:///var/run/docker.sock").
func New(host, apiVersion string) (*Client, error) {
	base, transport, err := baseFromHost(host, apiVersion)
	if err != nil {
		return nil, err
	}
	return &Client{
		http: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		base: base,
	}, nil
}

// NewWithBase construye un Client contra una base URL HTTP arbitraria.
// Útil en pruebas con httptest.Server.
func NewWithBase(base string) *Client {
	return &Client{
		http: &http.Client{Timeout: 10 * time.Second},
		base: strings.TrimRight(base, "/"),
	}
}

// baseFromHost resuelve el endpoint HTTP y el transporte a partir del host.
func baseFromHost(host, apiVersion string) (string, *http.Transport, error) {
	// Docker exige el prefijo /v<version> en la ruta (ej. /v1.41/_ping).
	apiPrefix := apiVersion
	if apiPrefix != "" && !strings.HasPrefix(apiPrefix, "v") {
		apiPrefix = "v" + apiPrefix
	}
	switch {
	case strings.HasPrefix(host, "unix://"):
		sock := strings.TrimPrefix(host, "unix://")
		tr := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", sock)
			},
		}
		return "http://docker/" + apiPrefix, tr, nil
	case strings.HasPrefix(host, "http://"), strings.HasPrefix(host, "https://"):
		base := strings.TrimRight(host, "/")
		if apiPrefix != "" && !strings.Contains(base, apiPrefix) {
			base += "/" + apiPrefix
		}
		return base, &http.Transport{}, nil
	default:
		return "", nil, fmt.Errorf("docker: host no soportado %q", host)
	}
}

// Ping implementa ContainerAPI.Ping.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/_ping", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("docker: ping: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("docker: ping: estado %d", resp.StatusCode)
	}
	return nil
}

// ListManaged implementa ContainerAPI.ListManaged.
func (c *Client) ListManaged(ctx context.Context, labelKey, labelValue string) ([]Container, error) {
	filter := map[string][]string{
		"label": {labelKey + "=" + labelValue},
	}
	fb, _ := json.Marshal(filter)
	url := c.base + "/containers/json?all=0&filters=" + urlEscape(string(fb))
	resp, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var list []Container
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("docker: list: %w", err)
	}
	return list, nil
}

// Inspect implementa ContainerAPI.Inspect.
func (c *Client) Inspect(ctx context.Context, id string) (Container, error) {
	url := c.base + "/containers/" + id + "/json"
	resp, err := c.get(ctx, url)
	if err != nil {
		return Container{}, err
	}
	defer resp.Body.Close()
	var info struct {
		ID    string `json:"Id"`
		Name  string `json:"Name"`
		State struct {
			Status string `json:"Status"`
			PID    int    `json:"Pid"`
		} `json:"State"`
		Config struct {
			Image  string            `json:"Image"`
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return Container{}, fmt.Errorf("docker: inspect %s: %w", id, err)
	}
	return Container{
		ID:      info.ID,
		Name:    info.Name,
		Image:   info.Config.Image,
		State:   info.State.Status,
		Labels:  info.Config.Labels,
		HostPID: info.State.PID,
	}, nil
}

// Kill implementa ContainerAPI.Kill.
func (c *Client) Kill(ctx context.Context, id, signal string) error {
	url := c.base + "/containers/" + id + "/kill?signal=" + urlEscape(signal)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("docker: kill %s: %w", id, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("docker: kill %s: estado %d", id, resp.StatusCode)
	}
	return nil
}

func (c *Client) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("docker: estado %d en %s", resp.StatusCode, url)
	}
	return resp, nil
}

func urlEscape(s string) string {
	// Docker acepta valores escapados por QueryEscape; mantenemos la
	// compatibilidad sustituyendo espacios por %20 y comillas.
	replacer := strings.NewReplacer(" ", "%20", `"`, "%22", "{", "%7B", "}", "%7D", ":", "%3A", ",", "%2C", "[", "%5B", "]", "%5D")
	return replacer.Replace(s)
}
