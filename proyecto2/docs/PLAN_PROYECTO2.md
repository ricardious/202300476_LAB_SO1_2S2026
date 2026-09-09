# Plan maestro — Proyecto 2 de Sistemas Operativos 1

## 1. Meta del proyecto

Construir, demostrar y documentar un sistema Linux de telemetría y gestión autónoma de
contenedores que cubra el 100 % de la rúbrica. El identificador oficial será el carnet
`202300476` y la interfaz del módulo será:

```text
/proc/continfo_pr2_so1_202300476
```

Fecha límite de implementación indicada en el enunciado: **17 de septiembre de 2026**.

## 2. Principios de ejecución

- Todo el código y la documentación nuevos vivirán dentro de `proyecto2/`.
- Las interfaces entre componentes se fijan antes de implementarlos.
- El trabajo técnico se realiza con OpenCode y `opencode-go/deepseek-v4-flash`.
- Los commits los crea un agente OpenCode independiente con
  `opencode-go/muse-spark-1.3-contributor`.
- Los agentes de implementación no hacen commits.
- Cada commit debe ser pequeño, compilable cuando aplique y usar Conventional Commits.
- Ninguna rama se integra sin pruebas, revisión contra la rúbrica y árbol Git limpio.
- Se priorizan los 100 puntos obligatorios; el módulo Rust queda como mejora opcional.

## 3. Arquitectura decidida

```text
Linux kernel
  ├─ módulo C ───────────────> /proc/continfo_pr2_so1_202300476 (JSON)
  └─ eBPF tracepoint kill ───> ring buffer ─┐
                                             │
Docker Engine API <──────── daemon Go <──────┘
                           ├─ política de conservación/eliminación
                           ├─ instala y retira cronjob
                           └─ escribe métricas/eventos en Valkey
                                                        │
                                                        v
                                             Grafana + Redis datasource
```

### 3.1 Módulo de kernel en C

El módulo usará `seq_file` para exponer un documento JSON consistente. Iterará procesos
con las primitivas seguras del kernel, tomará referencias apropiadas y evitará mantener
bloqueos mientras formatea la salida. El documento raíz tendrá este contrato:

```json
{
  "schema_version": 1,
  "timestamp_unix_ns": 0,
  "memory": {
    "total_kb": 0,
    "free_kb": 0,
    "used_kb": 0
  },
  "processes": [
    {
      "pid": 0,
      "name": "",
      "cmdline": "",
      "container_id": "",
      "vsz_kb": 0,
      "rss_kb": 0,
      "memory_percent": 0.0,
      "cpu_percent": 0.0
    }
  ]
}
```

Se incluirán procesos generales y procesos de contenedores. La identificación de
contenedor aprovechará la información disponible de cgroups y conservará `cmdline` como
alternativa verificable. Los cálculos documentarán unidades, redondeo y limitaciones por
versión del kernel. Los scripts de carga y descarga serán idempotentes y comprobarán la
creación/eliminación del archivo `/proc`.

### 3.2 Daemon en Go

El daemon tendrá paquetes separados para configuración, lectura/validación del JSON de
`/proc`, cliente Docker, política de selección, almacenamiento Valkey, consumidor eBPF,
cronjob y ciclo de vida. El `main` solo conectará dependencias y controlará señales.

Flujo de inicio:

1. Validar configuración, privilegios, Docker, `/proc` y Valkey.
2. Ejecutar el script idempotente que compila/carga el módulo.
3. Levantar Valkey y Grafana mediante Compose si no están disponibles.
4. Instalar el cronjob marcado del proyecto, sin borrar entradas ajenas.
5. Cargar y adjuntar eBPF.
6. Ejecutar el ciclo de reconciliación cada 20 segundos por defecto.

Flujo de cada ciclo:

1. Leer y deserializar una instantánea completa de `/proc`.
2. Consultar Docker e identificar contenedores administrados por etiquetas.
3. Enriquecer cada contenedor con PID del host y métricas del kernel.
4. Persistir la métrica del sistema y los consumos históricos en Valkey.
5. Ordenar candidatos por RAM, VSZ, RSS y CPU con desempate determinista.
6. Mantener como mínimo 3 contenedores de bajo consumo y 2 de alto consumo.
7. Excluir siempre Grafana, Valkey y cualquier contenedor con etiqueta protegida.
8. Solicitar terminación de los candidatos excedentes.
9. Registrar una eliminación únicamente después de recibir el evento eBPF correlacionado.

Al recibir `SIGINT` o `SIGTERM`, el daemon detendrá el loop, retirará solamente su entrada
de cron y liberará recursos eBPF. No eliminará datos ni contenedores protegidos.

### 3.3 eBPF

Se usará un programa CO-RE con libbpf enlazado al tracepoint estable
`syscalls:sys_enter_kill`; se documentará y probará una alternativa compatible si el
kernel no publica ese tracepoint. Cada evento incluirá timestamp, PID/TGID emisor, PID
receptor y señal. Un ring buffer transportará eventos al daemon.

Antes de matar un contenedor, el daemon almacenará temporalmente la relación entre ID de
contenedor y PID del host. Solo un evento eBPF correspondiente confirmará la eliminación y
actualizará el contador/stream en Valkey. Los timeouts y eventos no relacionados quedarán
en logs, sin producir falsos positivos.

### 3.4 Valkey y Grafana

Valkey conservará claves con prefijo `so1:p2:`:

- `so1:p2:system:latest`: hash de la instantánea más reciente.
- `so1:p2:system:timeline`: stream temporal de RAM.
- `so1:p2:containers:timeline`: stream histórico de contenedores activos e inactivos.
- `so1:p2:rank:ram` y `so1:p2:rank:cpu`: sorted sets para rankings históricos.
- `so1:p2:kills`: stream de eliminaciones confirmadas por eBPF.
- `so1:p2:kills:count`: contador total confirmado.

Compose levantará Valkey y Grafana, instalará/provisionará el datasource de Redis/Valkey y
cargará automáticamente el dashboard. No se dependerá de configuración manual para la
demostración.

Paneles mínimos:

- RAM total, usada y libre.
- Evolución temporal de RAM.
- Eliminaciones confirmadas a lo largo del tiempo.
- Top 5 histórico por RAM con PID e ID.
- Top 5 histórico por CPU con PID e ID.
- Tabla/contador de eventos `kill` capturados por eBPF.

### 3.5 Generación de carga

Habrá imágenes reproducibles y etiquetadas para cuatro perfiles:

- `memory-high`: consumo alto y acotado de RAM.
- `cpu-high`: carga alta de CPU con límites configurables.
- `low`: consumo mínimo durante al menos cuatro minutos.
- `intruder`: intento controlado de lectura de archivos sensibles montados para la prueba;
  nunca leerá archivos reales del host fuera de un fixture explícito.

Cada dos minutos, el cronjob ejecutará un script que selecciona cinco perfiles de forma
aleatoria, aplica límites de recursos, etiquetas y nombres únicos. El script será seguro
ante ejecuciones simultáneas y permitirá semilla fija para pruebas.

## 4. Estructura objetivo

```text
proyecto2/
├── README.md
├── Makefile
├── .env.example
├── kernel/
│   ├── continfo.c
│   └── Makefile
├── ebpf/
│   ├── kill_monitor.bpf.c
│   ├── kill_monitor.h
│   └── Makefile
├── daemon/
│   ├── cmd/daemon/main.go
│   ├── internal/{config,procfs,docker,policy,valkey,ebpf,cron,lifecycle}/
│   ├── go.mod
│   └── go.sum
├── workloads/
│   ├── memory-high/
│   ├── cpu-high/
│   ├── low/
│   └── intruder/
├── deploy/
│   ├── docker-compose.yml
│   └── grafana/provisioning/{datasources,dashboards}/
├── scripts/
│   ├── load-module.sh
│   ├── unload-module.sh
│   ├── generate-containers.sh
│   ├── install-cron.sh
│   ├── remove-cron.sh
│   ├── demo.sh
│   └── doctor.sh
├── tests/{fixtures,integration}/
├── docs/
│   ├── Proyecto2-SO1-2doSem.pdf
│   ├── PLAN_PROYECTO2.md
│   ├── MANUAL_TECNICO.md
│   ├── GUIA_INSTALACION.md
│   ├── GUIA_DEMO.md
│   └── DEFENSA.md
└── evidencias/{kernel,daemon,ebpf,grafana,integracion}/
```

## 5. DAG, ramas y responsables

La rama actual `ricardious/proyecto2-setup` es la base. Las ramas de trabajo serán hijas de
esa base una vez que Muse confirme los documentos iniciales.

| Orden | Rama | Trabajo DeepSeek | Dependencia | Resultado |
|---:|---|---|---|---|
| 0 | `ricardious/proyecto2-setup` | Ninguno: el coordinador crea el plan | Ninguna | Enunciado y plan versionados |
| 1A | `ricardious/p2-kernel` | Módulo C, Makefile, scripts y pruebas | 0 | `/proc` correcto y módulo idempotente |
| 1B | `ricardious/p2-daemon` | Daemon, parser, Docker, política, Valkey y tests | 0 | Servicio Go comprobable con fixtures |
| 1C | `ricardious/p2-observability` | Carga, cron, Compose, Valkey y Grafana | 0 | Entorno reproducible y dashboard provisionado |
| 2 | `ricardious/p2-ebpf` | Programa eBPF y puente Go | 1A, 1B | Confirmación real de `kill` |
| 3 | `ricardious/p2-integration` | Integrar ramas y corregir contratos | 1A, 1B, 1C, 2 | Flujo extremo a extremo |
| 4 | `ricardious/p2-quality` | Revisión rúbrica, robustez y seguridad | 3 | Cero brechas obligatorias conocidas |
| 5 | `ricardious/p2-docs` | Manuales, demo, defensa y evidencias | 4 | Entrega y defensa preparadas |

En cada rama, DeepSeek deja cambios sin commit. Después, Muse inspecciona el diff, ejecuta
las verificaciones pertinentes y crea varios commits lógicos. Muse también realizará las
integraciones para que el historial refleje con claridad qué se incorporó y cuándo.

## 6. Secuencia prevista de commits

El número final dependerá de la implementación real; la meta es **24–32 commits útiles**,
no commits vacíos ni divisiones artificiales.

1. `docs(p2): add project statement`
2. `docs(p2): add rubric-driven implementation plan`
3. `build(kernel): add module build system`
4. `feat(kernel): expose system memory through procfs`
5. `feat(kernel): expose process telemetry through procfs`
6. `fix(kernel): harden process traversal and cleanup`
7. `test(kernel): add module lifecycle checks`
8. `chore(daemon): initialize Go service`
9. `feat(daemon): parse and validate procfs snapshots`
10. `feat(daemon): discover labeled Docker containers`
11. `feat(policy): preserve required workload quotas`
12. `feat(policy): rank removable containers deterministically`
13. `feat(valkey): persist telemetry and historical rankings`
14. `feat(lifecycle): install and clean project cron entry`
15. `test(daemon): cover parser policy and storage contracts`
16. `feat(workloads): add reproducible load images`
17. `feat(cron): generate labeled containers every two minutes`
18. `feat(deploy): add Valkey and Grafana compose stack`
19. `feat(grafana): provision datasource and dashboard`
20. `feat(ebpf): trace kill syscall events`
21. `feat(daemon): consume and correlate eBPF confirmations`
22. `test(ebpf): add event correlation verification`
23. `feat(integration): wire service startup and shutdown`
24. `test(integration): add end-to-end smoke workflow`
25. `chore(tooling): add doctor demo and cleanup scripts`
26. `docs(p2): add installation guide`
27. `docs(p2): add technical manual`
28. `docs(p2): add demonstration runbook`
29. `docs(p2): add oral defense guide`
30. `docs(evidence): record rubric verification results`

## 7. Estrategia de pruebas

### 7.1 Pruebas sin privilegios

- `go test ./...`, incluyendo fixtures válidos, truncados y con campos desconocidos.
- Race detector y análisis estático del daemon.
- Pruebas deterministas de ordenamiento, empates, mínimos y protección.
- Validación de scripts con `shellcheck` cuando esté disponible.
- Validación de Compose y JSON del dashboard.
- Compilación eBPF y del módulo contra headers disponibles.

### 7.2 Pruebas privilegiadas en VM Linux

- Compilar, cargar y descargar el módulo repetidamente.
- Verificar permisos, lectura y eliminación del archivo `/proc`.
- Comparar RAM reportada con `/proc/meminfo` dentro de una tolerancia documentada.
- Crear perfiles de carga y comprobar PID, nombre/cmdline, VSZ, RSS, RAM y CPU.
- Confirmar que siempre sobreviven al menos 3 bajos y 2 altos.
- Confirmar que Grafana y Valkey nunca son candidatos.
- Ejecutar una eliminación y demostrar: solicitud del daemon, evento eBPF y escritura Valkey.
- Reiniciar el daemon y demostrar idempotencia de cron, módulo, Compose y dashboard.
- Enviar `SIGTERM` y comprobar que solo se retira el cronjob del proyecto.

### 7.3 Prueba de demostración

`scripts/demo.sh` preparará el entorno y mostrará comandos breves, pero las evidencias se
capturarán también por separado para que el evaluador pueda verificar cada criterio.

## 8. Matriz de trazabilidad de la rúbrica

| Criterio | Implementación | Verificación | Evidencia requerida |
|---|---|---|---|
| 1.1 Compilación módulo | `kernel/Makefile` | `make -C proyecto2/kernel` | salida limpia y `.ko` |
| 1.2 Carga/descarga | scripts idempotentes | `insmod`, `lsmod`, `rmmod` | terminal + `dmesg` |
| 1.3 Interfaz `/proc` | `continfo.c` con `seq_file` | lectura + JSON válido + eliminación | contenido y ciclo de vida |
| 1.4 Daemon compila/ejecuta | `daemon/` | build, tests y ejecución | binario + logs |
| 1.5 `/proc` + Valkey | paquetes `procfs`/`valkey` | consulta a ambas interfaces | logs + `valkey-cli` |
| 2.1 Iteración `task_struct` | módulo C | procesos conocidos presentes | código explicado + salida |
| 2.2 PID/memoria/CPU | módulo C | contraste con herramientas del SO | tabla comparativa |
| 2.3 Parseo Go | parser tipado | tests de fixtures + claves Valkey | pruebas verdes |
| 3.1 Captura `kill` | programa eBPF | señal controlada | evento con PID/señal |
| 3.2 eBPF → Go → Valkey | ring buffer + correlación | eliminación completa | cadena de tres evidencias |
| 4.1 Manuales | `docs/` | checklist documental | PDFs/Markdown legibles |
| 4.2 Comprensión | `DEFENSA.md` | ensayo de preguntas | respuestas y diagramas |

## 9. Criterios de aceptación globales

- Todos los componentes obligatorios compilan sin errores en la VM objetivo.
- La interfaz `/proc` aparece al cargar y desaparece al descargar el módulo.
- El daemon tolera lecturas inválidas, desconexiones y apagado ordenado.
- Ninguna eliminación se registra como confirmada sin evento eBPF correlacionado.
- El sistema conserva 3 contenedores bajos y 2 altos y protege Grafana/Valkey.
- El cronjob crea cinco contenedores cada dos minutos y se elimina al finalizar.
- El dashboard se provisiona sin pasos manuales y contiene todos los paneles solicitados.
- Existe evidencia reproducible por cada fila de la rúbrica.
- El árbol Git final está limpio, las ramas están integradas y el historial es explicable.

## 10. Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| APIs internas del kernel varían | detectar versión/headers y encapsular compatibilidad |
| CPU de proceso es acumulativa | documentar fórmula y probar diferencias entre muestras |
| PID de contenedor usa namespaces | consultar PID del host mediante Docker Inspect |
| `kill` lo emite Docker/runc | correlacionar por PID receptor, señal y ventana temporal |
| Grafana no lee Valkey de fábrica | provisionar plugin/datasource y fijar versión compatible |
| Cron duplicado tras reinicio | entrada delimitada por marcadores y scripts idempotentes |
| Carga agota la VM | límites Docker, timeouts y script de limpieza |
| Pruebas requieren root | separar unitarias de integración y usar una VM objetivo |
| Evidencia incompleta | capturar resultados según matriz, no al final de memoria |

## 11. Cronograma operativo

| Fecha | Hito |
|---|---|
| 8–9 sep | Plan, contratos y base Git |
| 9–11 sep | Kernel, daemon y entorno de observabilidad en paralelo |
| 11–12 sep | eBPF y correlación con el daemon |
| 13–14 sep | Integración privilegiada en VM y correcciones |
| 15 sep | Revisión exhaustiva contra la rúbrica |
| 16 sep | Manuales, dashboard final y evidencias |
| 17 sep | Ensayo de demo/defensa, limpieza y entrega |

## 12. Checklist de defensa oral

- Explicar por qué `/proc` usa `seq_file` y cómo se evita una lectura inconsistente.
- Explicar `task_struct`, VSZ frente a RSS y la fórmula de porcentajes.
- Dibujar el recorrido de una métrica desde kernel hasta Grafana.
- Justificar cómo se identifica un contenedor y su PID en el host.
- Explicar el algoritmo de ranking y cómo se garantizan los mínimos 3/2.
- Demostrar por qué Grafana y Valkey nunca se eliminan.
- Explicar tracepoint, verifier, mapa/ring buffer y evento de `kill`.
- Mostrar por qué Valkey solo registra una eliminación confirmada por eBPF.
- Explicar idempotencia, señales y limpieza del cronjob.
- Reconocer limitaciones de namespaces, versiones del kernel y medición de CPU.

## 13. Mejora opcional después del núcleo

Solo después de satisfacer y evidenciar los 100 puntos obligatorios se abrirá una rama
`ricardious/p2-rust-extra` para el módulo Rust que imprima `Hola Mundo 202300476`. Esta
actividad no debe retrasar pruebas, documentación ni evidencias del sistema principal.
