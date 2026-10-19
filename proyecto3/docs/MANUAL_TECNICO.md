# Manual técnico del Proyecto 3

## Propósito y arquitectura

El sistema procesa reportes militares ficticios de cinco países y presenta estadísticas en tiempo real. El perfil es completamente local: un nodo K3s en k3d, Docker como anfitrión del nodo, Zot como proceso externo al clúster y Locust como proceso del host. No se usan recursos de GCP ni cuentas de facturación. El anuncio del curso permite Gateway API local con CRDs estándar y un controlador compatible; se eligió Envoy Gateway.

Los reportes entran por Gateway y HTTPRoute en `/grpc-202300476`. La API Rust valida el JSON y llama por REST a Go Ingest. Ese Deployment tiene dos contenedores: API REST y cliente gRPC, comunicados por localhost. El cliente invoca `WarReportService.SendReport` en un Service headless. Cada pod del Deployment Go Writer contiene un servidor gRPC y un Writer HTTP, también comunicados por localhost. El Writer publica en RabbitMQ y espera confirmación del broker antes de responder.

Un Consumer Go recibe mensajes con prefetch 16, valida, agrega en Valkey y confirma después de almacenar. Su endpoint `/metrics` consulta las estadísticas de Valkey y las expone en formato Prometheus. Prometheus muestrea cada cinco segundos; Grafana consulta ese datasource integrado. El dashboard se refresca cada diez segundos y no requiere plugins de terceros.

El flujo opcional `/dapr-202300476` pasa por la misma API Rust hacia Go con el SDK oficial de Dapr, sidecar pub/sub RabbitMQ y suscriptor que almacena en Valkey. Comparte el modelo de datos y la deduplicación con el flujo gRPC. Se detalla su implementación compacta, garantías y pruebas en [Dapr](DAPR.md).

Los diagramas y contratos se encuentran en el README, `proto/openapi.yaml` y `proto/reports.proto`. El campo interno `report_id` se añade sin cambiar los cuatro campos del JSON de entrada.

## Interfaz y validación

```json
{
  "country": "ESP",
  "warplanes_in_air": 42,
  "warships_in_water": 14,
  "timestamp": "2026-10-19T19:25:00Z"
}
```

Países aceptados: USA, RUS, CHN, ESP y GTM. Aviones entre 0 y 50; barcos entre 0 y 30; fecha RFC3339. Rust rechaza campos adicionales, valores fuera de rango y cuerpos mayores de 4096 bytes. Las APIs internas también validan mensajes para no confiar únicamente en la primera capa.

`Idempotency-Key` es opcional y permite reintentar el mismo reporte. Sin encabezado utilizable se genera un UUID. Reutilizar el mismo identificador para datos diferentes no crea un segundo reporte: la identidad corresponde al primer mensaje almacenado durante la ventana de deduplicación.

La respuesta 202 con `status:queued` significa que RabbitMQ confirmó la publicación persistente, no que Grafana ya deba mostrar el reporte. El almacenamiento y scraping son asíncronos. Errores de la cadena devuelven 502 o 503; solicitudes inválidas producen errores 4xx.

La ruta pública es HTTP REST aunque se llame `/grpc-202300476`; el transporte gRPC ocurre entre los servicios Go. Gateway API no se sustituye por Ingress ni por un proxy programado manualmente.

## RabbitMQ y garantías

El broker utiliza almacenamiento persistente y un nombre de nodo estable. La cola `reports` es durable; los mensajes se publican con DeliveryMode persistente, identificador y confirmación. El Writer usa una conexión reutilizada y serializa la publicación por canal; al perder la conexión vuelve a declararla. Las devoluciones por publicaciones sin ruta se tratan como errores.

El consumidor usa ACK manual después de completar la actualización de Valkey. Si Valkey falla se hace NACK con reencolado y una pausa para evitar un bucle de CPU. Un mensaje inválido se rechaza sin reencolar y pasa a `reports.dead` mediante `reports.dlx`. El Consumer se reconecta cuando RabbitMQ vuelve a estar disponible.

La entrega es al menos una vez. La deduplicación en Valkey evita el doble conteo durante 24 horas; no se presenta como una garantía universal de entrega exactamente una vez. La cola impone TTL de 24 horas y un máximo de 10000 mensajes listos, con rechazo de nuevas publicaciones al llenarse. La cola de descarte se limita a 1000 mensajes. La memoria y el disco son acotados para la demostración local; no se promete retención ilimitada.

## Modelo de datos en Valkey

Prefijo exclusivo `so1:p3:` en base 0. Una secuencia Lua agrupa la comprobación de identidad y las actualizaciones de las estadísticas, evitando intercalados entre consumidores. Los tipos de claves existentes se verifican antes de escribir.

| Clave | Tipo | Contenido |
|---|---|---|
| `stats` | Hash | Conteo global, conteos por país, máximos y mínimos de cada unidad. |
| `planes:hist` y `ships:hist` | Hash | Frecuencias exactas para calcular la moda. |
| `planes:rank` y `ships:rank` | Sorted set | Suma acumulada de unidades reportadas por país. |
| `latest:ESP` y equivalentes | Hash | Último evento por timestamp, aviones y barcos. |
| `timeline` | Stream | Últimos 10000 reportes crudos como máximo. |
| `seen:<report_id>` | String con TTL | Identificadores almacenados durante 24 horas. |

La moda conserva todos los valores empatados en la etiqueta `ties`; el panel muestra esos valores y un representante numérico menor. Máximos, mínimos y ranking son acumulados desde el inicio del conjunto de datos; el selector temporal de Grafana afecta a las series, no redefine esos acumulados. Top 5 suma unidades reportadas: no identifica aeronaves o barcos físicos únicos.

Los eventos fuera de orden incrementan los acumulados, pero no reemplazan `latest` si su timestamp es anterior. Las series temporales de Grafana son muestras del último estado observado cada cinco segundos. No constituyen un archivo de cada evento individual.

Valkey usa AOF con fsync cada segundo y PVC. El proceso limita datos a 64 MB y usa `noeviction`; al agotarse la memoria se aplica contrapresión a través del Consumer y la cola, en lugar de eliminar estadísticas silenciosamente. AOF cada segundo tolera el reinicio normal probado; un fallo abrupto del sistema puede perder las escrituras aún no sincronizadas.

## Helm Operator

Se generó el proyecto con Operator SDK v1.42.3 y plugin Helm. `watches.yaml` asocia `monitoring.so1.local/v1alpha1` y los kinds `ValkeyInstance` y `GrafanaInstance` con sus respectivos charts. La imagen del operador añade charts propios al runtime oficial de Helm Operator.

Los Custom Resources son independientes. El primero crea StatefulSet, Service y PVC de Valkey; el segundo crea Deployment, Service, PVC y provisioning de Grafana. El controlador compara estado deseado y real, instala/actualiza releases Helm y observa dependencias. La reconciliación periódica es de un minuto. El Deployment del operador usa concurrencia uno y RBAC limitado al namespace.

El chart de Grafana incorpora un checksum del provisioning en su pod template. Al cambiar datasource o dashboard se reinicia el pod para renovar los archivos montados por subPath. Los volúmenes persistentes se conservan al retirar recursos para prevenir pérdida accidental.

`ValkeyInstance.spec.replica.enabled:true` habilita una réplica real de lectura con `--replicaof`; el Service `valkey-read` dirige las lecturas a ella. El primario sigue recibiendo todas las escrituras. La configuración diaria usa `false` para ahorrar RAM.

## Gateway y Zot

Envoy Gateway instala los CRDs estándar y su controlador. GatewayClass `so1-envoy` referencia el controlador de Envoy; Gateway `military` escucha HTTP 80 y HTTPRoute `reports` envía la ruta exacta a `api-rust:8080`. El Service generado para Envoy es interno al clúster y se abre un túnel en localhost 8088. La entrada continúa atravesando Gateway y HTTPRoute; el túnel solamente permite acceder al Service desde el equipo.

Zot está fuera del clúster, escucha en puerto 5005 y usa autenticación y HTTPS con una CA local. Los manifiestos de los componentes del proyecto utilizan imágenes de Zot. K3s resuelve el registry lógico `localhost:5005` mediante un mirror HTTPS hacia `host.k3d.internal:5005`, con la CA y credenciales locales. Los componentes internos de K3s y su propia imagen de nodo proceden de sus registros de distribución.

Las imágenes Docker v2 copiadas de upstream se convierten a manifiestos OCI cuando Zot lo requiere; los blobs conservan sus digests. Los binarios propios se empaquetan con Podman en OCI. No se usa `skip_verify` ni publicación por HTTP.

El archivo de entrada `load-profile.json` se publica como artifact OCI `application/vnd.so1.load-profile.v1+json`, se descarga desde Zot y configura países, rangos y semilla de Locust. `make artifact` realiza publicación, descarga y comparación byte a byte.

## HPA y pruebas de réplicas

Rust solicita 50m de CPU y escala de una a tres réplicas con objetivo de utilización 30%. El porcentaje es relativo a la CPU solicitada, no al procesador completo. Metrics Server suministra las medidas. La prueba de carga observó el incremento real de una a tres réplicas; `hpa-observado.txt` conserva la secuencia.

Locust genera reportes válidos y únicos; el perfil normal incluye espera de 0.5 a 1.5 segundos. Empezar con pocos usuarios. Las ráfagas para comparación quitaron esa espera y usaron 20 usuarios durante 45 segundos.

Se compararon uno y dos pods de Writers; se reinició Go Ingest entre ensayos para renovar la resolución DNS de ambos endpoints gRPC. El cliente usa round robin. Los resultados y limitaciones del experimento están en [evidencias](evidencias/README.md). En un único nodo, más réplicas distribuyen trabajo entre procesos pero no añaden hardware. Una réplica de Valkey no escala las escrituras del primario.

## Falco

Falco 0.45.0 usa Modern eBPF. La única regla personalizada detecta aperturas de lectura del archivo exclusivo de prueba. El archivo no contiene secretos reales y existe en las imágenes de Rust y Go.

```bash
source scripts/env.sh
k exec deploy/api-rust -- cat /app/secretos/prueba.txt
k logs daemonset/falco --tail=20
```

La regla completa se encuentra en `deploy/falco-values.yaml`; las alertas reales están en `docs/evidencias/falco.txt`. Registran archivo, comando y PID. El perfil desactiva collectors de metadatos de contenedor y descargas automáticas de reglas para ahorrar recursos; no se afirma que incluya reglas generales de seguridad adicionales.

El kernel local presenta advertencias sobre tracepoints opcionales de mitigación TOCTOU y namespaces PID anidados. La captura exigida se probó efectivamente. Falco necesita margen durante la carga de eBPF: su máximo de 768 MiB permite el arranque; su consumo estable observado es de decenas de MiB. El límite inicial menor provocaba OOM y fue corregido.

## Preparación de la evaluación

Antes del bloque, levantar todo con `make up` y comprobar `make status`. Tener Grafana y Locust abiertos, con el catálogo de Zot accesible tras confiar la CA. No compilar ni desplegar durante el turno.

```bash
source scripts/env.sh
kubectl get nodes -o wide
k get gateway,httproute
k get hpa,deployments
k logs deploy/api-rust --tail=10
kubectl get crd
k get valkeyinstances,grafanainstances
k get pods
k exec valkey-0 -- valkey-cli DBSIZE
k logs daemonset/falco --tail=20
```

El carnet es 202300476, por lo que el país asignado es ESP. Explicar la comunicación entre los dos contenedores de cada Deployment Go, el significado de 202, los ACK y confirmaciones, la deduplicación, el operador y las limitaciones del equipo.

El repositorio debe permanecer privado y dar acceso a los auxiliares indicados por el enunciado. La entrega en UEDI y Classroom cuando corresponda es responsabilidad del estudiante; los scripts no envían mensajes ni publican el repositorio. Dapr se incluye como extra y puede pausarse con `make dapr-off` para reducir el consumo diario.

## Referencias

- [K3s y requisitos](https://docs.k3s.io/installation/requirements)
- [Configuración de registros privados de K3s](https://docs.k3s.io/installation/private-registry)
- [Envoy Gateway](https://gateway.envoyproxy.io/docs/)
- [Operator SDK Helm](https://sdk.operatorframework.io/docs/building-operators/helm/quickstart/)
- [Falco Modern eBPF](https://falco.org/docs/concepts/event-sources/kernel/)

Las pruebas y mediciones verifican el entorno local descrito; no equivalen a un despliegue en nube ni a tolerancia a fallos de varios equipos.
