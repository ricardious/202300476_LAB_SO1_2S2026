# Operador de Valkey y Grafana

Operador de Kubernetes basado en Helm, generado con Operator SDK v1.42.3. Administra el almacenamiento y la visualización del Proyecto 3 mediante dos recursos personalizados del grupo `monitoring.so1.local/v1alpha1`.

## Recursos administrados

| Recurso personalizado | Recursos de Kubernetes | Configuración principal |
|---|---|---|
| `ValkeyInstance` | StatefulSet, Service y PVC; réplica de lectura opcional. | `spec.resources`, `spec.image`, `spec.storage`, `spec.maxmemory` y `spec.replica.enabled`. |
| `GrafanaInstance` | Deployment, Service, PVC y provisioning de datasource y dashboard. | `spec.resources`, `spec.image` y `spec.adminSecret`. |

Las instancias utilizadas por el proyecto se llaman `valkey` y `grafana`. Cada recurso personalizado corresponde a un chart en `helm-charts/`; las asociaciones se definen en [watches.yaml](watches.yaml). El archivo [PROJECT](PROJECT) conserva la configuración de generación del operador.

## Reconciliación y persistencia

El controlador observa los recursos personalizados y sus dependencias, instala o actualiza las releases Helm y registra las condiciones de estado en los CR. Además de las notificaciones de cambios, realiza una reconciliación periódica cada minuto.

`ValkeyInstance.spec.replica.enabled: true` crea una réplica de lectura configurada mediante `--replicaof` y un Service `valkey-read`. Las escrituras se dirigen al primario. Al deshabilitar la réplica, su PVC se conserva conforme a la política del StatefulSet. El PVC de Grafana utiliza la política de retención `keep` de Helm.

Las pruebas de reconciliación y replicación se documentan en [las evidencias del proyecto](../docs/evidencias/README.md).

## Despliegue

La imagen extiende el runtime oficial Helm Operator e incorpora los charts del proyecto. El manifiesto [operator.yaml](../deploy/operator.yaml) define el Deployment, la cuenta de servicio y el RBAC limitado al namespace `so1-p3`. Los archivos restantes de `config/` corresponden a la estructura generada por Operator SDK.

La construcción y el despliegue se realizan desde `proyecto3/`:

```bash
make build
make up
```

## Administración

Desde `proyecto3/`, consultar las instancias y los logs del controlador:

```bash
source scripts/env.sh
k get valkeyinstances,grafanainstances
k logs deploy/p3-operator --tail=20
```

Habilitar la réplica de lectura de Valkey y verificar su enlace con el primario:

```bash
k patch valkeyinstance valkey --type=merge -p '{"spec":{"replica":{"enabled":true}}}'
k wait --for=create statefulset/valkey-replica --timeout=120s
k rollout status statefulset/valkey-replica
k exec valkey-replica-0 -- valkey-cli INFO replication
```

La salida esperada incluye `role:slave` y `master_link_status:up`. Para volver a la configuración de una sola instancia:

```bash
k patch valkeyinstance valkey --type=merge -p '{"spec":{"replica":{"enabled":false}}}'
```

La réplica permite distribuir lecturas; la capacidad de escritura continúa dependiendo del primario.
