# 202300476_LAB_SO1_2S2026

**Proyecto 1 · Sistemas Operativos 1 · 2do Semestre 2026**
Universidad de San Carlos de Guatemala — Facultad de Ingeniería

> **Carnet:** 202300476

---

## Objetivo

Diseñar e implementar un entorno virtualizado (KVM) que integre máquinas virtuales y
contenedores para simular un entorno real de desarrollo: **3 APIs REST en Go**
contenedorizadas, un **registro privado de imágenes (Zot)** y **comunicación cruzada**
entre servicios mediante REST/HTTP con JSON.

## Arquitectura

3 máquinas virtuales sobre KVM, cada una con un runtime de contenedores distinto:

```
              HOST (KVM)
      ┌───────────┼───────────┐
     VM1         VM2         VM3
 Containerd    Podman      Docker
      │           │           │
  API1, API2    API3     Zot Registry
  :8081,:8082   :8083       :5000
```

## Tabla de VMs

| VM | Hostname | IP | Runtime | Contenedores |
|---|---|---|---|---|
| SO1-VM1 | so1-vm1 | 192.168.100.10 | Containerd | API1, API2 |
| SO1-VM2 | so1-vm2 | 192.168.100.20 | Podman | API3 |
| SO1-VM3 | so1-vm3 | 192.168.100.30 | Docker | Zot Registry |

## Tecnologías

- **Go** (APIs) · **KVM/qemu-kvm** (virtualización) · **Containerd** · **Podman** ·
  **Docker** · **Zot** (registro) · REST/HTTP + JSON

## Estructura del repositorio

```
proyecto1/
├── api1/  api2/  api3/     # Proyectos Go independientes (código + Dockerfile + tests)
├── infra/                  # Red, VMs y networking
├── scripts/                # build-images.sh + validation/
├── docs/                   # Manual, guía, requisitos y evidencias
│   └── evidencias/         # Capturas y salidas de validación
├── README.md
└── .gitignore
```

## Cómo ejecutar

1. **Construir imágenes:**
   ```bash
   bash scripts/build-images.sh
   ```
2. **Push/Pull al registro Zot** (detalle en la guía de instalación §9).
3. **Desplegar** (detalle en la guía de instalación §10):
   - VM1 (containerd): `ctr -n default run --detach --net-host --env ... <img> api1/api2`
   - VM2 (podman): `podman run -d -p 8083:8083 -e ... <img> api3`

## Endpoints

| API | Puerto | `/health` | Llamada 1 | Llamada 2 |
|---|---|---|---|---|
| API1 | 8081 | `/health` | `/api1/202300476/call-api2` | `/api1/202300476/call-api3` |
| API2 | 8082 | `/health` | `/api2/202300476/call-api1` | `/api2/202300476/call-api3` |
| API3 | 8083 | `/health` | `/api3/202300476/call-api1` | `/api3/202300476/call-api2` |

`/health` → `{"status":"UP","message":"API# is Ready","timestamp":"...","VM":"...","carnet":"202300476"}`

## Variables de entorno

| Variable | Función |
|---|---|
| `PORT` | Puerto de escucha (default 8081/8082/8083) |
| `TARGET_API1/2/3` | URL `/health` de cada API destino (evita IPs hardcodeadas) |

Valores reales desplegados: ver `docs/GUIA_INSTALACION.md` §10.

## Zot Registry

- URL: `http://192.168.100.30:5000`
- Catálogo:
  ```bash
  curl http://192.168.100.30:5000/v2/_catalog
  # {"repositories":["api1-202300476","api2-202300476","api3-202300476"]}
  ```
- Config: `/opt/zot/config.yaml` · Datos: `/opt/zot/data`

## Pruebas

```bash
bash scripts/validation/test-all.sh                # /health + 6 endpoints
bash scripts/validation/test-all.sh --error-test   # + prueba de error
bash scripts/validation/validate-registry.sh       # catálogo Zot
```

---

## Documentación

- [Manual técnico](docs/MANUAL_TECNICO.md)
- [Guía de instalación](docs/GUIA_INSTALACION.md)
