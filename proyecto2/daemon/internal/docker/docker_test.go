package docker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListManagedAndInspectAndKill(t *testing.T) {
	var killed string
	var killedSignal string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/_ping":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/containers/json":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]Container{{
				ID:    "abc123",
				Names: []string{"/memory-high"},
				Image: "stress:latest",
				State: "running",
			}})
		case strings.HasPrefix(r.URL.Path, "/containers/abc123/json"):
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Id":    "abc123",
				"Name":  "/memory-high",
				"State": map[string]interface{}{"Status": "running", "Pid": 120},
				"Config": map[string]interface{}{
					"Image":  "stress:latest",
					"Labels": map[string]string{"so1.p2.managed": "1"},
				},
			})
		case strings.HasPrefix(r.URL.Path, "/containers/abc123/kill"):
			killed = "abc123"
			killedSignal = r.URL.Query().Get("signal")
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	cli := NewWithBase(srv.URL)
	ctx := context.Background()

	if err := cli.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	managed, err := cli.ListManaged(ctx, "so1.p2.managed", "1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(managed) != 1 || managed[0].ID != "abc123" {
		t.Fatalf("managed = %+v", managed)
	}

	insp, err := cli.Inspect(ctx, "abc123")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if insp.HostPID != 120 {
		t.Errorf("HostPID = %d, esperaba 120", insp.HostPID)
	}
	if insp.DisplayName() != "memory-high" {
		t.Errorf("DisplayName = %q", insp.DisplayName())
	}

	if err := cli.Kill(ctx, "abc123", "SIGKILL"); err != nil {
		t.Fatalf("kill: %v", err)
	}
	if killed != "abc123" || killedSignal != "SIGKILL" {
		t.Errorf("kill = %s sig %s", killed, killedSignal)
	}
}

func TestPingFailsOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cli := NewWithBase(srv.URL)
	if err := cli.Ping(context.Background()); err == nil {
		t.Error("ping con estado 404 debe fallar")
	}
}

func TestBaseFromHostUnix(t *testing.T) {
	base, tr, err := baseFromHost("unix:///var/run/docker.sock", "1.41")
	if err != nil {
		t.Fatalf("baseFromHost: %v", err)
	}
	if base != "http://docker/1.41" {
		t.Errorf("base = %q", base)
	}
	if tr == nil {
		t.Error("transporte unix no debe ser nil")
	}
}

func TestBaseFromHostHTTP(t *testing.T) {
	base, _, err := baseFromHost("http://localhost:2375", "1.41")
	if err != nil {
		t.Fatalf("baseFromHost: %v", err)
	}
	if base != "http://localhost:2375/1.41" {
		t.Errorf("base = %q", base)
	}
}

func TestBaseFromHostInvalid(t *testing.T) {
	if _, _, err := baseFromHost("tcp://foo", "1.41"); err == nil {
		t.Error("host no soportado debe dar error")
	}
}

func TestDisplayNameFallback(t *testing.T) {
	c := Container{ID: "id1"}
	if c.DisplayName() != "id1" {
		t.Errorf("DisplayName = %q, esperaba id1", c.DisplayName())
	}
	c2 := Container{Name: "/named"}
	if c2.DisplayName() != "named" {
		t.Errorf("DisplayName = %q, esperaba named", c2.DisplayName())
	}
}
