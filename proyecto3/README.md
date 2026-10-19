# Proyecto 3 M U M N K8s

Sistema local de reportes militares para Sistemas Operativos 1, carnet **202300476**. País asignado: **ESP España**. Implementa el flujo completo con Rust, Go, gRPC, RabbitMQ, Valkey, Grafana, Gateway API, Helm Operator, Locust, Zot, Falco y el extra Dapr.

```text
Locust → Gateway API Envoy → API Rust
                            ↓ REST
                       Go API + gRPC Client
                            ↓ gRPC
                       Go Server + Writer → RabbitMQ → Consumer → Valkey
                                                                    ↓
                                                        exporter → Prometheus → Grafana
Helm Operator → ValkeyInstance + GrafanaInstance
Zot HTTPS externo al clúster → imágenes y perfil OCI
Falco Modern eBPF → alerta al leer el archivo de prueba
Ruta /dapr-202300476 → Rust → Go SDK → daprd → RabbitMQ → suscriptor SDK → Valkey
```

El perfil usa **k3d con un nodo K3s en Docker**, sin VM ni servicios de pago. La elección evita instalar servicios del sistema con sudo. Los datos voluminosos se guardan en `/mnt/fedora-extra/so1-proyecto3`, fuera de este repositorio. Se usa Podman con un almacén independiente para construir las imágenes. No se modifica el Docker de otros proyectos.

## Instalación inicial

Requisitos del host y explicación completa en [la guía de instalación](docs/GUIA_INSTALACION.md).

```bash
cd proyecto3
make bootstrap       # herramientas fijadas, certificados, Zot y clúster
make setup-load      # Locust en un venv independiente
make mirror          # imágenes externas convertidas a OCI y publicadas en Zot
make build           # compilación secuencial y publicación de imágenes propias
make up              # recursos, operador, Gateway, Falco y accesos locales
make test
make e2e
make load USERS=5 DURATION=30s
```

Los cinco pasos de instalación y despliegue también pueden ejecutarse con `make setup`. GNU Make se configura para no paralelizar tareas pesadas.

## Uso diario

```bash
make up              # reanudar y desplegar
make tunnels         # restablecer accesos si se reemplazó un pod
make status
make doctor
make credentials     # muestra credenciales locales; no publicar su salida
make load-ui         # interfaz en segundo plano, sin iniciar tráfico
make evidence
make compare         # repetir comparación de Writers y réplica de lectura
make resilience      # comprobar recuperación y persistencia en recursos propios
make dapr-test       # comprobar SDK pub/sub, deduplicación y recuperación
make compare-flows   # comparar gRPC y Dapr con carga moderada
make dapr-off        # ahorrar el consumo del pod opcional
make dapr-on         # volver a habilitarlo
make down            # detener el proyecto conservando datos
```

| Interfaz | Dirección |
|---|---|
| Grafana | http://127.0.0.1:3003/d/so1-p3-202300476 |
| Locust | http://127.0.0.1:8089 después de `make load-ui` |
| API a través del Gateway | POST http://127.0.0.1:8088/grpc-202300476 |
| Flujo opcional Dapr | POST http://127.0.0.1:8088/dapr-202300476 |
| Zot | https://localhost:5005/v2/_catalog |

Grafana y Zot usan el usuario `so1`; las contraseñas se generan localmente. Para Zot, importar como autoridad confiable el certificado `certs/ca.crt` del directorio de datos. Las herramientas ya usan esa CA explícitamente. No se desactiva la validación TLS.

## Consumo acotado

- Nodo limitado a 3584 MiB, sin VM y sin nodos adicionales.
- Una réplica por componente; Rust escala entre 1 y 3 con objetivo de CPU 30%.
- Go usa un hilo lógico por proceso y pools pequeños; Rust usa dos workers.
- Valkey limita datos a 64 MB, sin expulsar datos silenciosamente; AOF y PVC conservan el conjunto.
- Cola máxima de 10000 mensajes pendientes; mensajes con TTL de 24 horas y confirmación del broker.
- Historial crudo máximo de 10000 reportes; Prometheus retiene 2 horas y hasta 128 MB de bloques.
- Logs de pods con rotación de 5 MiB y dos archivos.
- Falco ejecuta una sola regla y comparte buffers entre CPU. Su límite de 768 MiB permite el pico de arranque; el uso estable medido es mucho menor.

## Documentación y evidencias

- [Manual técnico](docs/MANUAL_TECNICO.md)
- [Guía de instalación](docs/GUIA_INSTALACION.md)
- [Evidencias y análisis de rendimiento](docs/evidencias/README.md)
- [Contrato REST](proto/openapi.yaml) y [contrato gRPC](proto/reports.proto)
- [Operador](operator/README.md)
- [Flujo Dapr, recursos y pruebas](docs/DAPR.md)

Dapr se incluye como extra con un solo pod adicional y sin plano de control propio. El entorno no simula ser GKE; usa la modalidad local indicada por el anuncio del curso.
