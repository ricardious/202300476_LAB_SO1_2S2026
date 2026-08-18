package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const carnet = "202300476"

// API2 vive en VM1 y es responsable de llamar a API1 y API3.
var (
	apiName = "API2"
	vmID    = "VM1"
	port    = "8082"
	client  = &http.Client{Timeout: 5 * time.Second}
)

type target struct {
	Name string
	VM   string
	URL  string
}

type healthResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	VM        string `json:"VM"`
	Carnet    string `json:"carnet"`
}

type callResponse struct {
	APIName    string `json:"apiname"`
	Message    string `json:"message"`
	Connection bool   `json:"connection"`
	Carnet     string `json:"carnet"`
}

// buildTargets lee las URLs /health de las APIs destino desde variables de entorno,
// de modo que las IPs no quedan hardcodeadas en el código.
func buildTargets() map[string]target {
	return map[string]target{
		"api1": {Name: "API1", VM: "VM1", URL: envOr("TARGET_API1", "")},
		"api3": {Name: "API3", VM: "VM2", URL: envOr("TARGET_API3", "")},
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	targets := buildTargets()
	mux := newRouter(targets)

	log.Printf("%s listening on :%s", apiName, port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

// newRouter registra los endpoints y devuelve el mux (reutilizable en pruebas).
func newRouter(targets map[string]target) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	for key, t := range targets {
		key, t := key, t
		mux.HandleFunc(fmt.Sprintf("/%s/%s/call-%s", strings.ToLower(apiName), carnet, key), makeCallHandler(t))
	}
	return mux
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:    "UP",
		Message:   apiName + " is Ready",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		VM:        vmID,
		Carnet:    carnet,
	})
}

func makeCallHandler(t target) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		up, _ := checkTarget(t)
		resp := callResponse{
			APIName:    t.Name,
			Message:    fmt.Sprintf("The %s located on the %s is working", t.Name, t.VM),
			Connection: up,
			Carnet:     carnet,
		}
		if !up {
			resp.Message = fmt.Sprintf("ERROR: The %s located on the %s is not working", t.Name, t.VM)
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// checkTarget realiza una peticion HTTP real al /health de la API destino,
// espera la respuesta y verifica que status sea exactamente "UP".
func checkTarget(t target) (bool, error) {
	if t.URL == "" {
		return false, fmt.Errorf("no se configuro URL para %s (variable de entorno)", t.Name)
	}
	resp, err := client.Get(t.URL)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var h healthResponse
	if err := json.Unmarshal(body, &h); err != nil {
		return false, err
	}
	return h.Status == "UP", nil
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
