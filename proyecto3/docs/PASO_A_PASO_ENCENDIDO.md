# Encendido y demostración local del Proyecto 3

Guía de operación para el entorno ya instalado en la computadora de Ricardo, carnet **202300476**, país asignado **España (ESP)**. Los pasos de preparación de la evaluación siguen la Guía Proyecto 3 v2 recibida el 24 de octubre de 2026.

El entorno utiliza Docker, un nodo K3s administrado por k3d, Envoy Gateway, Zot local, los servicios Rust y Go, RabbitMQ, Valkey, Prometheus, Grafana y Falco. El flujo extra con Dapr está incluido. Los datos del proyecto están en `/mnt/fedora-extra/so1-proyecto3`.

## 1. Abrir una terminal Bash

Si la terminal inicia en Zsh, ejecutar primero:

```bash
bash
```

Entrar a la carpeta del proyecto:

```bash
cd /mnt/development/orca/workspaces/202300476_LAB_SO1_2S2026/pompano/proyecto3
```

Todos los comandos de esta guía parten de esa carpeta. Cada terminal nueva debe repetir estos pasos; las que usen `kubectl` o el comando abreviado `k` también deben cargar `scripts/env.sh`.

## 2. Revisar Docker y el espacio disponible

```bash
docker info --format 'Docker {{.ServerVersion}}'
df -h / /mnt/development /mnt/fedora-extra
free -h
```

Docker debe responder con su versión. Si indica que no puede conectarse al daemon, iniciar el servicio del sistema:

```bash
sudo systemctl start docker
```

Ese comando puede solicitar la contraseña de la cuenta de Linux. Después, repetir `docker info`.

Mantener preferentemente **4 GB o más libres en la raíz** y margen de al menos **1 GB en desarrollo**. Kubelet tiene un umbral de disco de 2 GiB y una reclamación mínima de 512 MiB: si se activa `DiskPressure`, recuperar espacio por encima de 2,5 GiB y verificar que la condición desaparezca. La partición externa también necesita espacio para imágenes y datos.

Cerrar programas pesados antes de usar el entorno completo. El nodo tiene un límite de 3584 MiB; Grafana, el navegador y Locust comparten la memoria de la computadora.

## 3. Encender el proyecto

```bash
make up
```

Esperar a que termine y comprobar que no haya errores. Este comando:

1. Inicia Zot con HTTPS y autenticación.
2. Inicia el clúster `so1-p3` y restaura el acceso del nodo al registro local.
3. Aplica los manifiestos y espera el arranque de los servicios.
4. Despliega Gateway API y Falco, y habilita el flujo Dapr.
5. Abre los túneles locales hacia Gateway y Grafana.
6. Deja Locust disponible, sin iniciar tráfico.

El encendido puede tardar varios minutos, especialmente después de reiniciar la computadora. Para la operación diaria basta `make up`; `make setup`, `make mirror` y `make build` corresponden a la instalación inicial o a cambios de imágenes. La [guía de instalación](GUIA_INSTALACION.md) documenta ese proceso.

**Durante el bloque de evaluación no ejecutar `make up`, `make build` ni comandos que creen o reinicien servicios. El despliegue debe estar terminado antes de entrar.**

## 4. Comprobar que los componentes estén listos

```bash
make status
source scripts/env.sh
kubectl get nodes -o wide
k get pods
k get gateway,httproute
k get hpa,deployments
k get valkeyinstances,grafanainstances
```

Comprobar:

- El nodo aparece como `Ready`.
- Los pods actuales están en `Running` y todos sus contenedores están listos: `1/1` o `2/2`, según el servicio.
- Gateway `military` tiene la condición `Programmed=True`.
- Existen `ValkeyInstance/valkey` y `GrafanaInstance/grafana`.
- El HPA de Rust puede obtener métricas; al arrancar pueden tardar en aparecer.

Los registros antiguos de pods `Evicted` o `Completed` no son los pods actuales. Para comprobar específicamente la presión de disco:

```bash
kubectl get node k3d-so1-p3-server-0 -o jsonpath='{range .status.conditions[?(@.type=="DiskPressure")]}{.type}={.status}{"\n"}{end}'
```

El resultado esperado es `DiskPressure=False`. Que el nodo esté `Ready` por sí solo no descarta presión de disco.

Revisar también Valkey y los registros de la API:

```bash
k exec valkey-0 -- valkey-cli DBSIZE
k logs deploy/api-rust --tail=10
```

`DBSIZE` devuelve la cantidad de claves, no el número de reportes.

## 5. Consultar las credenciales y abrir las pantallas

```bash
make credentials
```

El usuario de Grafana y Zot es **`so1`**. El comando muestra una contraseña para cada servicio; usar la correspondiente. No copiar la salida a capturas de evaluación ni al repositorio.

| Pantalla | Dirección |
| --- | --- |
| Locust | http://127.0.0.1:8089 |
| Catálogo de imágenes de Zot | https://localhost:5005/v2/_catalog |
| Consola de Grafana | http://127.0.0.1:3003/d/so1-p3-202300476?theme=dark&kiosk |
| Grafana con navegación normal | http://127.0.0.1:3003/d/so1-p3-202300476?theme=dark |

Zot utiliza una CA local. Si el navegador no la reconoce, importar `/mnt/fedora-extra/so1-proyecto3/certs/ca.crt` como autoridad de confianza antes de la evaluación. El catálogo solicita autenticación y muestra una lista JSON de repositorios.

En Grafana, iniciar sesión con el campo de usuario, sin dirección de correo. Si se conserva una versión anterior de la consola en caché, recargar con **Ctrl + Shift + R**.

El mapa permite seleccionar países. Las dos series inferiores siguen mostrando España, aunque se seleccione otro país en el mapa. Las estadísticas globales y los rankings son históricos; el selector de tiempo controla las series, no recalcula esos agregados. Una línea plana puede indicar que no han llegado nuevos reportes. Ver [lectura de los indicadores](GRAFANA.md).

## 6. Probar el flujo principal desde Locust

En http://127.0.0.1:8089 configurar:

| Campo | Valor inicial |
| --- | --- |
| Number of users | `3` |
| Spawn rate | `1` |
| Host | `http://127.0.0.1:8088` |
| report-path | `/grpc-202300476` |

Pulsar **Start**. Comprobar que aparezcan solicitudes y observar la columna de fallos. La ruta principal acepta reportes mediante `POST` y responde normalmente `202 Accepted`; abrirla en una pestaña como `GET` no prueba el flujo.

Abrir Grafana y esperar al menos un ciclo de actualización, de 10 segundos. Las estadísticas deberían actualizarse cuando el consumidor almacene los reportes. Al terminar, pulsar **Stop** en Locust para ahorrar recursos.

Se puede ejecutar una prueba breve desde la terminal, con Locust web detenido:

```bash
make load USERS=3 DURATION=30s
```

Este comando descarga el perfil OCI desde Zot y genera reportes durante 30 segundos. Escribe los resultados en `docs/evidencias/locust-manual_*`. No ejecutar dos generadores simultáneamente al realizar verificaciones de conteos.

## 7. Preparar y demostrar la alerta de Falco

La guía v2 solicita una alerta en vivo. Tener listas dos terminales Bash, ambas en `proyecto3` y con el entorno cargado.

**Terminal de logs:**

```bash
source scripts/env.sh
k logs daemonset/falco --tail=10 -f
```

**Terminal que genera el evento:**

```bash
source scripts/env.sh
k exec deploy/api-rust -- cat /app/secretos/prueba.txt
```

La lectura del archivo de prueba debe producir una alerta con la regla **`SO1 Sensitive File Read`** y el texto `SO1 lectura sensible`. El archivo contiene texto de prueba, no credenciales.

Guardar una captura del registro con la alerta visible para el manual. `Ctrl + C` termina el seguimiento de logs; no apaga Falco. El YAML de la regla está en [`deploy/falco-values.yaml`](../deploy/falco-values.yaml).

## 8. Preparar el flujo extra Dapr

Dapr ya se habilita con `make up`. Para comprobar sus contenedores:

```bash
source scripts/env.sh
k get pods -l app=go-dapr
```

En Locust, detener la prueba anterior y cambiar `report-path` a `/dapr-202300476`. Iniciar otra prueba con los mismos valores y observar los resultados. Ambos flujos alimentan las mismas estadísticas de Valkey y Grafana.

Si Dapr se había pausado, reactivarlo **antes del bloque**:

```bash
make dapr-on
```

Para ahorrar recursos durante trabajo diario cuando no se vaya a usar el extra:

```bash
make dapr-off
```

La guía v2 considera Dapr un extra opcional. Ver [arquitectura y validación de Dapr](DAPR.md).

## 9. Dejar lista la evaluación

Antes del ingreso al bloque:

- [ ] Todo el sistema está desplegado; `make status` no muestra problemas en los componentes actuales.
- [ ] Terminal Bash con `scripts/env.sh` cargado y comandos de inspección preparados.
- [ ] Locust abierto y listo para iniciar carga.
- [ ] Catálogo de Zot abierto y autenticado por HTTPS.
- [ ] Grafana abierto con España seleccionada inicialmente.
- [ ] Logs de Falco abiertos y comando para disparar la alerta preparado.
- [ ] Comparaciones de 1 y 2 réplicas de Go Writers y Valkey documentadas.
- [ ] Manual técnico, guía de instalación, regla Falco y capturas accesibles.
- [ ] Entregas de UEDI y Classroom revisadas.

Las comparaciones ya tienen evidencias en `docs/evidencias/`: `locust-writer-1_*`, `locust-writer-2_*`, `valkey-primario.csv` y `valkey-replica.csv`. Su explicación está en el [índice de evidencias](evidencias/README.md). `make compare` modifica las réplicas y reinicia servicios: reservarlo para la preparación, fuera del bloque, con Locust web detenido.

La guía v2 describe grupos de cinco estudiantes y mantiene referencias a GKE y Zot en una VM de GCP. Esta instalación sigue la modalidad local del anuncio del curso compartido durante el desarrollo. Conservar ese anuncio junto con el material de evaluación para explicar el uso de k3d y Zot en el host.

## 10. Resolver problemas de arranque

### Kubernetes muestra DiskPressure

```bash
df -h / /mnt/fedora-extra
source scripts/env.sh
kubectl describe node k3d-so1-p3-server-0
```

Liberar cachés regenerables y recuperar el margen de disco. No borrar PVC, volúmenes ni `/mnt/fedora-extra/so1-proyecto3`. Esperar a que `DiskPressure=False` y comprobar los pods antes de repetir `make up`.

### Aparecen ErrImagePull o ImagePullBackOff

Revisar primero los eventos:

```bash
source scripts/env.sh
k get events --sort-by=.lastTimestamp
```

Después de un reinicio de Docker, `make bootstrap` restaura Zot y el alias local del registro:

```bash
make bootstrap
```

Volver a comprobar los pods. Si faltan imágenes que nunca se publicaron, seguir la guía de instalación; repetir el encendido no construye imágenes.

### Grafana o Gateway no responden en localhost

Reabrir los accesos:

```bash
make tunnels
curl --fail --max-time 5 http://127.0.0.1:3003/api/health
```

Si un pod fue reemplazado, el túnel anterior puede seguir asociado al pod viejo hasta que falle una conexión. Tras el fallo, repetir `make tunnels` y la consulta. Como recuperación completa, fuera del bloque de evaluación, `make down` seguido de `make up` recrea los accesos y conserva los datos.

### Locust no está abierto

```bash
make load-ui
```

Si el mensaje indica que falta el ejecutable, ejecutar `make setup-load` durante la preparación y volver a abrirlo. Si Zot no responde, resolver primero el acceso al registro.

### Necesito ver los registros de un componente

```bash
source scripts/env.sh
k logs deploy/api-rust --tail=30
k logs deploy/consumer --tail=30
k logs deploy/p3-operator --tail=30
k logs daemonset/falco --tail=30
```

## 11. Apagar al terminar

Detener la carga con **Stop** en Locust y ejecutar:

```bash
make down
```

La parada detiene el clúster del proyecto, sus túneles, Locust y Zot. Conserva imágenes, PVC, credenciales y datos persistentes, y mantiene los contenedores ajenos al proyecto. Para el siguiente uso, volver al paso 2 y ejecutar `make up`.

## Referencias

- Guía Proyecto 3 v2, recibida el 24 de octubre de 2026: `/home/ricardious/Descargas/Guía_proyecto3_v2.pdf`.
- [Guía de instalación local](GUIA_INSTALACION.md).
- [Manual técnico](MANUAL_TECNICO.md).
- [Consola de Grafana](GRAFANA.md).
