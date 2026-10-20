# Evidencias y análisis de pruebas

Las pruebas se ejecutaron en un entorno local con Fedora 42, procesador Ryzen 5 7520U y 7 GiB de RAM utilizables. El nodo k3d tiene un límite de 3584 MiB y comparte CPU y disco con las aplicaciones del host y el generador Locust.

## Locust y comparación de Writers

| Escenario | Solicitudes CSV | Fallos | Solicitudes por segundo | p50 ms | p95 ms | p99 ms |
|---|---:|---:|---:|---:|---:|---:|
| Carga básica 5 usuarios | 133 | 0 | 4.61 | 11 | 46 | 51 |
| Ráfaga 20 usuarios 1 Writer | 5015 | 0 | 113.77 | 110 | 350 | 390 |
| Ráfaga 20 usuarios 2 Writers | 8743 | 0 | 197.89 | 65 | 250 | 320 |

La carga básica duró 30 segundos con espera de 0.5 a 1.5 segundos por usuario. Las dos ráfagas duraron 45 segundos, con 20 usuarios, rampa de 4 usuarios por segundo y sin espera entre reportes. Se mantuvieron los mismos rangos de datos; las marcas de tiempo e identificadores son únicos. La versión actual del generador incorpora una semilla determinista por usuario; esta modificación es posterior a los ensayos de Writers registrados en esta sección.

Los CSV contienen las muestras exportadas por Locust. El cierre de solicitudes en curso puede producir diferencias respecto al resumen final de consola. Estos conteos corresponden a solicitudes HTTP, no al total de mensajes almacenados.

Con dos Writers se observó mayor tasa y menor latencia. Antes del segundo ensayo se reinició Go Ingest para que el cliente gRPC resolviera ambos endpoints del Service headless. Rust tenía HPA activo y el host compartía recursos con otros procesos. Los resultados describen el comportamiento del sistema completo bajo estas condiciones; el diseño del ensayo no permite aislar el efecto del Writer ni extrapolar capacidad entre máquinas.

## HPA

`hpa-observado.txt` registra el objetivo de CPU del 30%, un incremento de utilización hasta 144% y el paso de 1 a 3 réplicas de Rust. La utilización de CPU registrada corresponde al procesamiento del flujo de reportes. Al terminar, HPA vuelve progresivamente al mínimo de una réplica.

## Valkey primario y réplica de lectura

`valkey-primario.csv` y `valkey-replica.csv` contienen 1000 consultas HGETALL al estado agregado con cinco conexiones. Se habilitó la réplica mediante `ValkeyInstance.spec.replica.enabled`, se verificó `role:slave` y `master_link_status:up`, y se consultó el Service `valkey-read`.

El primario registró 6211.18 consultas/s y p95 de 0.199 ms; la réplica registró 3355.70 consultas/s y p95 de 0.327 ms. En esta prueba corta y en el mismo equipo no hubo mejora al añadir el proceso de réplica. Todas las escrituras se dirigen al primario. Al finalizar el ensayo se deshabilitó la réplica de lectura y se conservó su PVC.

## Persistencia y recuperación

`resiliencia.txt` verifica tres puntos: RabbitMQ acepta reportes con el Consumer detenido, el Consumer recupera exactamente los reportes pendientes y Valkey conserva el conteo después de reemplazar su pod usando el PVC y AOF. El ensayo preservó el conjunto de datos existente.

Las pruebas Go incluyen validación de país/rangos/timestamp, JSON extra o concatenado, empates de moda y una prueba de integración con Valkey en la base dedicada 15. Esa prueba verificó deduplicación, ranking, mínimos/máximos y que un evento fuera de orden no sobrescriba el último evento temporal. La prueba utilizó una base independiente del conjunto de datos de la demostración, almacenado en la base 0.

La prueba E2E envía dos veces el mismo identificador por Gateway y comprueba que aumenta una sola vez el conteo. También comprueba errores de validación y una ruta inexistente.

## Operador y Gateway

`operador.txt` contiene las condiciones de estado de ValkeyInstance y GrafanaInstance. Se verificó reconciliación del Service de Grafana después de eliminarlo y creación de la réplica Valkey por cambio del CR. `gateway.txt` registra Gateway Programmed y la ruta HTTP hacia Rust. La exposición pública se resuelve mediante Gateway API.

## Falco

`falco-alerta.jsonl` y `falco.txt` contienen alertas JSON con la regla `SO1 Sensitive File Read`, el archivo `/app/secretos/prueba.txt`, el comando cat y el PID. La configuración y regla están en `../../deploy/falco-values.yaml`.

En este kernel, Falco informa que no están disponibles algunos tracepoints adicionales de mitigación TOCTOU y desactiva BPF iterators por el namespace PID anidado. La validación cubre la regla personalizada de lectura del archivo de prueba; las funciones adicionales afectadas por estas restricciones quedan fuera del alcance del ensayo. Se desactivaron los recolectores de metadatos de contenedor para reducir dependencias; la regla usa la ruta exclusiva de prueba y registra PID y comando.

El límite inicial de 256/384 MiB causaba OOM durante la carga de eBPF. El límite se ajustó a 768 MiB para cubrir el arranque. En ejecución estable, Kubernetes registró un consumo de unas decenas de MiB; el límite representa el máximo permitido al contenedor.

## Capturas y estado

- `reinicio.txt`: parada y arranque completos mediante los comandos públicos del proyecto.
- `zot-catalogo.json`: catálogo obtenido por HTTPS con CA y autenticación verificadas.
- `locust.png`: prueba iniciada y detenida desde la interfaz web, sin errores.
- `dashboard.png`: dashboard de Grafana con métricas procedentes de Valkey vía exporter y Prometheus.
- `estado-kubernetes.txt`, `recursos.txt`, `contenedores.txt`: pods y consumo medido del nodo.
- `memoria-host.txt`, `disco.txt`, `ESPACIO_INICIAL.md`: memoria y preparación del almacenamiento.
- `oci-artifact.txt`: publicación y descarga verificadas del perfil de carga desde Zot.

Las cifras de consumo corresponden a muestras puntuales del nodo. El navegador, el generador, Zot y los compiladores se ejecutan en el host y requieren memoria adicional. Las pruebas iniciales mostraron alrededor de 1.5–1.6 GiB de uso del contenedor nodo en reposo, con los componentes del sistema activos.

## Flujo Dapr: validación y comparación

Las pruebas del flujo Dapr se realizaron el 19 de octubre de 2026 sobre `/dapr-202300476` con runtime Dapr 1.18.7 y SDK Go 1.13.0. `dapr-metadata.json` confirma el runtime, el componente RabbitMQ y la suscripción programática al topic `military-reports-dapr`; `dapr-suscripcion.json` contiene la respuesta del SDK a `/dapr/subscribe`.

`dapr-unitarias.txt` registra pruebas del handler: ACK al almacenar o deduplicar, reintento ante indisponibilidad de almacenamiento y descarte de mensajes inválidos. `dapr-e2e.txt` verifica el recorrido por Gateway, deduplicación compartida entre ambas rutas, validación, continuidad del flujo gRPC con Dapr apagado y recuperación de un reporte aceptado mientras Valkey estaba detenido. Los recursos pausados se restauraron y la cola Dapr terminó vacía. `dapr-aplicacion.txt` conserva errores transitorios de almacenamiento, posterior `stored=true` y `stored=false` para la repetición del mismo identificador.

La comparación de `dapr-comparacion.txt` usa **5 usuarios, 30 segundos por ruta**, rampa de un usuario por segundo, espera de 0.5–1.5 segundos y el mismo perfil OCI/semillas. Primero se ejecutó gRPC y después Dapr, sin generadores concurrentes.

| Flujo | Solicitudes CSV | Fallos | Solicitudes/s | p50 ms | p95 ms | p99 ms |
|---|---:|---:|---:|---:|---:|---:|
| gRPC | 135 | 0 | 4.66 | 12 | 45 | 60 |
| Dapr pub/sub | 132 | 0 | 4.55 | 11 | 50 | 57 |

Los resultados proceden de `compare-grpc_stats.csv` y `compare-dapr_stats.csv`; los resúmenes finales de consola pueden incluir solicitudes adicionales completadas durante el cierre. Ambos flujos procesaron la carga del ensayo sin errores. Su corta duración, el hardware compartido y la actividad de HPA limitan la interpretación de las diferencias de latencia y no permiten determinar capacidad máxima.

Al terminar la comparación, Kubernetes midió **8 MiB para Go y 25 MiB para daprd**, aproximadamente 33 MiB en el pod adicional. Una muestra posterior registró 8 + 28 MiB. Los archivos `dapr-recursos-antes.txt` y `dapr-recursos-despues.txt` registran consumo y estado del nodo. La diferencia entre ambas muestras incluye variaciones de caché, actividad del broker y sustitución de pods durante el despliegue, por lo que no aísla el consumo atribuible a Dapr. El límite del nodo continúa siendo 3.5 GiB.

`dapr-locust-ui.json` verifica una prueba de cuatro solicitudes Dapr iniciada mediante el selector de ruta de la interfaz Locust y detenida sin errores. La prueba finalizó con cero usuarios activos. La arquitectura, las garantías y los comandos `dapr-on`/`dapr-off` se detallan en [Dapr](../DAPR.md).
