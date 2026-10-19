# Evidencias y análisis de pruebas

Estas evidencias proceden de ejecuciones reales en la computadora local, no de un clúster de nube. Hardware: Ryzen 5 7520U, 7 GiB de RAM utilizables y Fedora 42. El nodo k3d tiene un límite de 3584 MiB y comparte CPU y disco con el escritorio, las herramientas y el generador Locust.

## Locust y comparación de Writers

| Escenario | Solicitudes CSV | Fallos | Solicitudes por segundo | p50 ms | p95 ms | p99 ms |
|---|---:|---:|---:|---:|---:|---:|
| Carga básica 5 usuarios | 133 | 0 | 4.61 | 11 | 46 | 51 |
| Ráfaga 20 usuarios 1 Writer | 5015 | 0 | 113.77 | 110 | 350 | 390 |
| Ráfaga 20 usuarios 2 Writers | 8743 | 0 | 197.89 | 65 | 250 | 320 |

La carga básica duró 30 segundos con espera de 0.5 a 1.5 segundos por usuario. Las dos ráfagas duraron 45 segundos, con 20 usuarios, rampa de 4 usuarios por segundo y sin espera entre reportes. Se mantuvieron los mismos rangos de datos; las marcas de tiempo e identificadores son únicos. La implementación del generador se dejó después con semilla determinista por usuario para repetir perfiles futuros.

Los CSV son los snapshots exportados por Locust; pueden diferir ligeramente del resumen final de consola por solicitudes en vuelo durante el cierre. No se presentan como el total exacto de mensajes almacenados.

Con dos Writers se observó mayor tasa y menor latencia. Antes del segundo ensayo se reinició Go Ingest para que el cliente gRPC resolviera ambos endpoints del Service headless. Rust tenía HPA activo y el host compartía recursos con otros procesos; por tanto, son ensayos descriptivos del sistema completo, no un experimento que aísle únicamente el efecto del Writer. No prueban escalabilidad entre máquinas ni un rendimiento garantizado.

## HPA

`hpa-observado.txt` registra el objetivo de CPU del 30%, un incremento de utilización hasta 144% y el paso de 1 a 3 réplicas de Rust. No se generó carga de CPU artificial dentro de la API; la actividad provino del flujo de reportes. Al terminar, HPA vuelve progresivamente al mínimo de una réplica.

## Valkey primario y réplica de lectura

`valkey-primario.csv` y `valkey-replica.csv` contienen 1000 consultas HGETALL al estado agregado con cinco conexiones. Se habilitó la réplica mediante `ValkeyInstance.spec.replica.enabled`, se verificó `role:slave` y `master_link_status:up`, y se consultó el Service `valkey-read`.

El primario registró 6211.18 consultas/s y p95 de 0.199 ms; la réplica registró 3355.70 consultas/s y p95 de 0.327 ms. En esta prueba corta y en el mismo equipo no hubo mejora al añadir el proceso de réplica. No se afirma escalado de escritura: todas las escrituras siguen dirigidas al primario. Se volvió al perfil sin réplica para ahorrar memoria; su PVC se conserva.

## Persistencia y recuperación

`resiliencia.txt` verifica tres puntos: RabbitMQ acepta reportes con el Consumer detenido, el Consumer recupera exactamente los reportes pendientes y Valkey conserva el conteo después de reemplazar su pod usando el PVC y AOF. El ensayo preservó el conjunto de datos existente.

Las pruebas Go incluyen validación de país/rangos/timestamp, JSON extra o concatenado, empates de moda y una prueba real contra Valkey en la base dedicada 15. Esa prueba verificó deduplicación, ranking, mínimos/máximos y que un evento fuera de orden no sobrescriba el último evento temporal. La base 0 de la demostración no se vació.

La prueba E2E envía dos veces el mismo identificador por Gateway y comprueba que aumenta una sola vez el conteo. También comprueba errores de validación y una ruta inexistente.

## Operador y Gateway

`operador.txt` contiene las condiciones reales de ValkeyInstance y GrafanaInstance. Se verificó reconciliación del Service de Grafana después de eliminarlo y creación de la réplica Valkey por cambio del CR. `gateway.txt` registra Gateway Programmed y la ruta HTTP hacia Rust. Ningún Ingress convencional implementa la ruta pública.

## Falco

`falco-alerta.jsonl` y `falco.txt` incluyen alertas JSON reales con la regla `SO1 Sensitive File Read`, el archivo `/app/secretos/prueba.txt`, el comando cat y el PID. La configuración y regla están en `../../deploy/falco-values.yaml`.

En este kernel, Falco informa que no están disponibles algunos tracepoints adicionales de mitigación TOCTOU y desactiva BPF iterators por el namespace PID anidado. La detección solicitada se verificó mediante alertas efectivas; no se afirma cobertura completa de todas las funciones de Falco. Se desactivaron los recolectores de metadatos de contenedor para reducir dependencias; la regla usa la ruta exclusiva de prueba y registra PID y comando.

El límite inicial de 256/384 MiB causaba OOM durante la carga de eBPF. Se dejó un máximo de 768 MiB para el arranque; la memoria estable medida por Kubernetes fue de unas decenas de MiB. Ese máximo no es una reserva permanente.

## Capturas y estado

- `reinicio.txt`: parada y arranque completos mediante los comandos públicos del proyecto.
- `zot-catalogo.json`: catálogo obtenido por HTTPS con CA y autenticación verificadas.
- `locust.png`: prueba real iniciada y detenida desde el navegador, sin errores.
- `dashboard.png`: dashboard real de Grafana con métricas procedentes de Valkey vía exporter y Prometheus.
- `estado-kubernetes.txt`, `recursos.txt`, `contenedores.txt`: pods y consumo medido del nodo.
- `memoria-host.txt`, `disco.txt`, `ESPACIO_INICIAL.md`: memoria y preparación del almacenamiento.
- `oci-artifact.txt`: publicación y descarga verificadas del perfil de carga desde Zot.

Las cifras de consumo son muestras puntuales. No incluyen una garantía de memoria máxima del host; navegador, generador, Zot y compiladores se ejecutan fuera del nodo. Las pruebas iniciales mostraron alrededor de 1.5–1.6 GiB de uso del contenedor nodo en reposo, con los componentes del sistema activos.

## Extra Dapr: implementación y comparación real

El 19 de octubre se añadió y verificó `/dapr-202300476` con runtime Dapr 1.18.7 y SDK Go 1.13.0. `dapr-metadata.json` confirma el runtime, el componente RabbitMQ y la suscripción programática al topic `military-reports-dapr`; `dapr-suscripcion.json` contiene la respuesta del SDK a `/dapr/subscribe`.

`dapr-unitarias.txt` registra pruebas del handler: ACK al almacenar o deduplicar, reintento ante indisponibilidad de almacenamiento y descarte de mensajes inválidos. `dapr-e2e.txt` verifica el recorrido por Gateway, deduplicación compartida entre ambas rutas, validación, continuidad del flujo gRPC con Dapr apagado y recuperación de un reporte aceptado mientras Valkey estaba detenido. Los recursos pausados se restauraron y la cola Dapr terminó vacía. `dapr-aplicacion.txt` conserva errores transitorios de almacenamiento, posterior `stored=true` y `stored=false` para la repetición del mismo identificador.

La comparación de `dapr-comparacion.txt` usa **5 usuarios, 30 segundos por ruta**, rampa de un usuario por segundo, espera de 0.5–1.5 segundos y el mismo perfil OCI/semillas. Primero se ejecutó gRPC y después Dapr, sin generadores concurrentes.

| Flujo | Solicitudes CSV | Fallos | Solicitudes/s | p50 ms | p95 ms | p99 ms |
|---|---:|---:|---:|---:|---:|---:|
| gRPC | 135 | 0 | 4.66 | 12 | 45 | 60 |
| Dapr pub/sub | 132 | 0 | 4.55 | 11 | 50 | 57 |

Son snapshots `compare-grpc_stats.csv` y `compare-dapr_stats.csv`; los resúmenes finales de consola pueden incluir solicitudes en vuelo adicionales. Esta prueba moderada demuestra funcionamiento sin errores, no capacidad máxima ni una ventaja de rendimiento de uno sobre otro. Las diferencias de milisegundos no son concluyentes en dos ejecuciones cortas sobre hardware compartido, con HPA y servicios persistentes activos.

Al terminar la comparación, Kubernetes midió **8 MiB para Go y 25 MiB para daprd**, aproximadamente 33 MiB en el pod adicional. Una muestra posterior registró 8 + 28 MiB. Los archivos `dapr-recursos-antes.txt` y `dapr-recursos-despues.txt` conservan consumo y estado del nodo: no deben usarse como una resta exacta del coste de Dapr, porque también cambiaron cachés, actividad del broker y pods durante el despliegue. El límite del nodo continúa siendo 3.5 GiB.

`dapr-locust-ui.json` verifica una prueba de cuatro solicitudes Dapr iniciada mediante el selector de ruta de la interfaz Locust y detenida sin errores. Al finalizar no queda carga activa. La arquitectura, las garantías y los comandos `dapr-on`/`dapr-off` se detallan en [Dapr](../DAPR.md).
