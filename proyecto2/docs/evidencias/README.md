# Evidencias de funcionamiento — Proyecto 2

Las siguientes capturas corresponden a ejecuciones reales del proyecto en
Fedora Linux con kernel `6.19.14-108.fc42.x86_64`. Demuestran la compilación y
el ciclo de vida del módulo, la ejecución del daemon, la generación de carga,
la captura de señales mediante eBPF, la persistencia en Valkey y la
visualización en Grafana.

## Módulo de kernel

### Compilación

El módulo `continfo.ko` se compiló contra los headers del kernel en ejecución.

![Compilación del módulo de kernel](kernel/01-compilacion-modulo.png)

### Carga e interfaz `/proc`

Después de cargar el módulo aparece
`/proc/continfo_pr2_so1_202300476` y `lsmod` confirma que `continfo` está activo.

![Carga del módulo e interfaz proc](kernel/02-carga-modulo-proc.png)

La interfaz entrega un documento JSON con memoria del sistema y datos de los
procesos.

![JSON generado por el módulo](kernel/03-json-procesos.png)

Los mensajes del kernel registran las operaciones de carga y descarga.

![Mensajes del módulo en dmesg](kernel/04-dmesg-modulo.png)

Finalmente, la descarga elimina tanto el módulo como su archivo en `/proc`.

![Descarga correcta del módulo](kernel/05-descarga-modulo.png)

## Daemon y política de contenedores

El daemon inicia con el módulo disponible y mantiene activo el consumidor
eBPF.

![Daemon con eBPF activo](daemon-politica/01-daemon-ebpf-activo.png)

La política reduce los contenedores gestionados y conserva los mínimos de tres
perfiles de bajo consumo y dos de alto consumo. Grafana y Valkey permanecen
protegidos.

![Aplicación de la política antes y después](daemon-politica/02-politica-antes-despues.png)

Detalle de la ejecución: [política y contenedores](daemon-politica/politica.txt).

## Cron y generación de carga

El cronjob de `root` ejecuta el generador cada dos minutos.

![Cronjob instalado](cron-workloads/01-cron-instalado.png)

Dos ejecuciones consecutivas generan diez contenedores con perfiles y límites
de recursos distintos.

![Generación de contenedores de carga](cron-workloads/02-generacion-diez-contenedores.png)

Detalle de la instalación: [cron y cargas](cron-workloads/cron.txt).

## Observabilidad eBPF

El objeto eBPF y el daemon se compilan correctamente.

![Compilación de eBPF y daemon](ebpf/01-compilacion-ebpf-daemon.png)

La prueba privilegiada adjunta los tracepoints y recibe el evento real de
`kill(2)`.

![Prueba privilegiada de eBPF](ebpf/02-prueba-privilegiada-ebpf.png)

Los eventos confirmados incrementan el contador y el stream de Valkey con PID,
señal, timestamp e identificador de contenedor.

![Correlación entre eBPF y Valkey](ebpf/03-correlacion-ebpf-valkey.png)

Datos completos: [eventos eBPF](ebpf/ebpf.txt).

## Valkey y Grafana

Valkey y Grafana se ejecutan como contenedores saludables.

![Servicios de Valkey y Grafana activos](valkey-grafana/01-stack-activo.png)

Las claves `so1:p2:*` contienen telemetría del sistema, timelines, rankings y
el historial de eliminaciones.

![Métricas almacenadas en Valkey](valkey-grafana/02-metricas-valkey.png)

### Kernel Operations Center

La vista general presenta capacidad de memoria, RAM usada y disponible,
procesos observados, presión de memoria y eliminaciones confirmadas.

![Vista general del dashboard](valkey-grafana/03-dashboard-overview.png)

La sección eBPF muestra el contador, la actividad en el tiempo y la bitácora de
señales confirmadas.

![Paneles de observabilidad eBPF](valkey-grafana/04-dashboard-ebpf.png)

El dashboard completa la observabilidad con rankings históricos de RAM y CPU,
además del timeline de contenedores activos e inactivos.

![Rankings y timeline de contenedores](valkey-grafana/05-dashboard-ranking-timeline.png)

Datos completos: [telemetría de Valkey y salud de Grafana](valkey-grafana/observabilidad.txt).

## Entorno de prueba

Las versiones de las herramientas, los headers del kernel y la disponibilidad
de BTF están documentados en [entorno.txt](entorno/entorno.txt).
