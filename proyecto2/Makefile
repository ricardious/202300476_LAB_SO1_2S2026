# ============================================================================
# Makefile de alto nivel — Proyecto 2 SO1 (Carnet 202300476)
#
# Objetivos principales:
#   make build   # compila el daemon Go (bin/so1-p2-daemon)
#   make test    # go test + pruebas shell/Compose (sin privilegios)
#   make up      # levanta Valkey + Grafana (Compose, proyecto so1-p2)
#   make down    # baja el stack Compose del proyecto (conserva volúmenes)
#   make install # instala daemon + generador en /usr/local/bin (root/sudo)
#   make run     # ejecuta el daemon en el HOST (lee .env)
#   make doctor  # diagnóstico del entorno
#   make demo    # demo de punta a punta
#   make evidence          # captura automática de evidencias (docs/evidencias/)
#   make verify-delivery   # preentrega estricta (0 sin fallos, 1 fallos, 2 uso)
#   make clean   # limpia artefactos de build (NO borra contenedores/volúmenes)
#
# Decision de despliegue: el daemon corre en el HOST (acceso a /proc del módulo
# y a eBPF con privilegios) mientras Compose ejecuta Valkey/Grafana. Ver
# README.md, sección "Despliegue".
# ============================================================================

SHELL := /usr/bin/env bash

DAEMON_DIR  := $(CURDIR)/daemon
EBPF_DIR    := $(CURDIR)/ebpf
KERNEL_DIR  := $(CURDIR)/kernel
DEPLOY_DIR  := $(CURDIR)/deploy
SCRIPTS_DIR := $(CURDIR)/scripts
BIN_DIR     := $(CURDIR)/bin
BIN         := $(BIN_DIR)/so1-p2-daemon
ENV_FILE    := $(CURDIR)/.env
COMPOSE     := $(DEPLOY_DIR)/docker-compose.yml
GENERATOR   := /usr/local/bin/so1-p2-generate-containers.sh

.PHONY: all build test up down install run doctor demo clean \
        ebpf module module-load module-unload install-cron remove-cron \
        lint vet fmt smoke purge evidence verify-delivery

all: build

# --------------------------------------------------------------------------
# build
# --------------------------------------------------------------------------
build:
	@mkdir -p $(BIN_DIR)
	cd $(DAEMON_DIR) && go build -o $(BIN) ./cmd/daemon
	@echo ">> daemon compilado en $(BIN)"

# --------------------------------------------------------------------------
# test (sin privilegios; los tests eBPF/módulo se omiten con mensaje claro)
# --------------------------------------------------------------------------
test:
	cd $(DAEMON_DIR) && go test ./...
	cd $(DAEMON_DIR) && go test -race ./...
	bash $(SCRIPTS_DIR)/../tests/integration/observability/run.sh
	bash $(SCRIPTS_DIR)/../tests/kernel/test-all.sh

# --------------------------------------------------------------------------
# up / down (Compose del proyecto; jamás toca recursos ajenos)
# --------------------------------------------------------------------------
up:
	docker compose -f $(COMPOSE) up -d --wait || docker compose -f $(COMPOSE) up -d
	@echo ">> Valkey/Grafana levantados (proyecto so1-p2)"

down:
	docker compose -f $(COMPOSE) down
	@echo ">> Stack so1-p2 abajo (volúmenes conservados)"

# --------------------------------------------------------------------------
# install (daemon + generador en /usr/local/bin; requiere root/sudo)
# --------------------------------------------------------------------------
install: build
	@mkdir -p /usr/local/bin
	@if [ -w /usr/local/bin ]; then \
		install -m 0755 $(BIN) /usr/local/bin/so1-p2-daemon; \
		ln -sfn $(SCRIPTS_DIR)/generate-containers.sh $(GENERATOR); \
	else \
		sudo install -m 0755 $(BIN) /usr/local/bin/so1-p2-daemon; \
		sudo ln -sfn $(SCRIPTS_DIR)/generate-containers.sh $(GENERATOR); \
	fi
	@echo ">> instalado: /usr/local/bin/so1-p2-daemon y $(GENERATOR) (symlink al repositorio)"

# --------------------------------------------------------------------------
# run (daemon en el HOST, con env de .env)
# --------------------------------------------------------------------------
run: build
	@set -a; \
	if [ -f $(ENV_FILE) ]; then . $(ENV_FILE); fi; \
	set +a; \
	cd $(CURDIR) && exec $(BIN)

# --------------------------------------------------------------------------
# doctor / demo
# --------------------------------------------------------------------------
doctor:
	bash $(SCRIPTS_DIR)/doctor.sh

demo:
	bash $(SCRIPTS_DIR)/demo.sh

# --------------------------------------------------------------------------
# evidencia / preentrega
# --------------------------------------------------------------------------
evidence:
	bash $(SCRIPTS_DIR)/capture-evidence.sh

verify-delivery:
	bash $(SCRIPTS_DIR)/verify-delivery.sh

# --------------------------------------------------------------------------
# frente eBPF y módulo de kernel
# --------------------------------------------------------------------------
ebpf:
	$(MAKE) -C $(EBPF_DIR) object
	$(MAKE) -C $(EBPF_DIR) bindings

module:
	$(MAKE) -C $(KERNEL_DIR)

module-load:
	bash $(SCRIPTS_DIR)/load-module.sh

module-unload:
	bash $(SCRIPTS_DIR)/unload-module.sh

install-cron:
	bash $(SCRIPTS_DIR)/install-cron.sh

remove-cron:
	bash $(SCRIPTS_DIR)/remove-cron.sh

# --------------------------------------------------------------------------
# calidad
# --------------------------------------------------------------------------
fmt:
	cd $(DAEMON_DIR) && gofmt -l .

vet:
	cd $(DAEMON_DIR) && go vet ./...

lint: fmt vet

# --------------------------------------------------------------------------
# clean — SOLO artefactos de build; no baja Compose ni borra datos.
# Para retirar recursos desplegados use: bash scripts/cleanup.sh
# --------------------------------------------------------------------------
clean:
	rm -rf $(BIN_DIR)
	cd $(DAEMON_DIR) && go clean ./...
	$(MAKE) -C $(EBPF_DIR) clean
	$(MAKE) -C $(KERNEL_DIR) clean
	@echo ">> artefactos de build eliminados (recursos desplegados intactos)"

# --------------------------------------------------------------------------
# purge — limpieza total SOLO de recursos del proyecto (con confirmación)
# --------------------------------------------------------------------------
purge:
	bash $(SCRIPTS_DIR)/cleanup.sh --module --purge
