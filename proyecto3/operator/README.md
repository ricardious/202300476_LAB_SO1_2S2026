# Operador de Valkey y Grafana

Generado con Operator SDK v1.42.3, plugin Helm. `PROJECT` conserva la configuración del scaffold y `watches.yaml` asocia dos kinds del grupo `monitoring.so1.local/v1alpha1` a sus charts propios. La imagen extiende el Helm Operator oficial y añade los charts; no sustituye el operador por scripts de kubectl.

`ValkeyInstance/valkey` administra un StatefulSet, Service y volumen persistente. `spec.resources`, `spec.image`, `spec.storage` y `spec.maxmemory` se trasladan a values. `spec.replica.enabled: true` añade una réplica de lectura real y su Service `valkey-read`; las escrituras siguen dirigidas al primario. Volver a `false` retira el proceso de réplica y conserva su PVC por la política de StatefulSet.

`GrafanaInstance/grafana` administra Deployment, Service, PVC, datasource Prometheus y dashboard. `spec.resources`, `spec.image` y `spec.adminSecret` controlan su configuración. El PVC se conserva mediante la política keep de Helm.

El runtime observa los Custom Resources y los recursos dependientes. Instala o actualiza releases Helm y registra condiciones en los CR. La reconciliación periódica es de un minuto, además de las notificaciones de cambios. Se verificó recuperación del Service de Grafana tras eliminarlo y creación de una réplica Valkey mediante el CR.

El despliegue utilizado es `../deploy/operator.yaml`, con ServiceAccount y Role limitados al namespace. Los directorios restantes de `config/` proceden del scaffold y sirven de referencia; no se utilizan como un segundo método de despliegue. Usar el Makefile de `proyecto3` para construir y desplegar.

```bash
source ../scripts/env.sh
k get valkeyinstances,grafanainstances
k logs deploy/p3-operator --tail=20
k patch valkeyinstance valkey --type=merge -p '{"spec":{"replica":{"enabled":true}}}'
k get statefulsets
# Al terminar la comparación:
k patch valkeyinstance valkey --type=merge -p '{"spec":{"replica":{"enabled":false}}}'
```

No aumentar `replicas` del primario para crear servidores independientes: eso no establece replicación. La prueba con dos procesos utiliza `--replicaof` y valida `master_link_status:up`.
