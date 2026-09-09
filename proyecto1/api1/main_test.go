package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func healthJSON(status string) string {
	h := healthResponse{
		Status:    status,
		Message:   apiName + " is Ready",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		VM:        vmID,
		Carnet:    carnet,
	}
	b, _ := json.Marshal(h)
	return string(b)
}

func TestHealth(t *testing.T) {
	router := newRouter(map[string]target{})
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, se esperaba 200", rec.Code)
	}
	var h healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &h); err != nil {
		t.Fatal(err)
	}
	if h.Status != "UP" {
		t.Errorf("status = %q, se esperaba UP", h.Status)
	}
	if h.Message != "API1 is Ready" {
		t.Errorf("message = %q, se esperaba 'API1 is Ready'", h.Message)
	}
	if h.VM != "VM1" {
		t.Errorf("VM = %q, se esperaba VM1", h.VM)
	}
	if h.Carnet != carnet {
		t.Errorf("carnet = %q, se esperaba %s", h.Carnet, carnet)
	}
	if h.Timestamp == "" {
		t.Error("timestamp vacio")
	}
}

func TestCallSuccess(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("path = %q, se esperaba /health", r.URL.Path)
		}
		w.Write([]byte(healthJSON("UP")))
	}))
	defer mock.Close()

	targets := map[string]target{
		"api2": {Name: "API2", VM: "VM1", URL: mock.URL + "/health"},
	}
	router := newRouter(targets)

	req := httptest.NewRequest("GET", "/api1/"+carnet+"/call-api2", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, se esperaba 200", rec.Code)
	}
	var c callResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	if !c.Connection {
		t.Errorf("connection = false, se esperaba true (API destino UP)")
	}
	if c.Message != "The API2 located on the VM1 is working" {
		t.Errorf("message = %q", c.Message)
	}
	if c.APIName != "API2" {
		t.Errorf("apiname = %q, se esperaba API2", c.APIName)
	}
	if c.Carnet != carnet {
		t.Errorf("carnet = %q", c.Carnet)
	}
}

func TestCallTargetDown(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	downURL := ts.URL
	ts.Close() // apagamos el destino: la conexion fallara

	targets := map[string]target{
		"api2": {Name: "API2", VM: "VM1", URL: downURL},
	}
	router := newRouter(targets)

	req := httptest.NewRequest("GET", "/api1/"+carnet+"/call-api2", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var c callResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &c)
	if c.Connection {
		t.Errorf("connection = true, se esperaba false (destino caido)")
	}
	if !strings.Contains(c.Message, "ERROR") {
		t.Errorf("message deberia contener ERROR, se obtuvo: %q", c.Message)
	}
	if c.Message != "ERROR: The API2 located on the VM1 is not working" {
		t.Errorf("message = %q", c.Message)
	}
}

func TestCallStatusNotUp(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(healthJSON("DOWN")))
	}))
	defer mock.Close()

	targets := map[string]target{
		"api3": {Name: "API3", VM: "VM2", URL: mock.URL},
	}
	router := newRouter(targets)

	req := httptest.NewRequest("GET", "/api1/"+carnet+"/call-api3", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var c callResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &c)
	if c.Connection {
		t.Errorf("connection = true, se esperaba false (status no UP)")
	}
	if !strings.Contains(c.Message, "ERROR") {
		t.Errorf("message deberia contener ERROR: %q", c.Message)
	}
}
