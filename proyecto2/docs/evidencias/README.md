# Evidencias — Proyecto 2 de SO1 (202300476)

Carpeta de evidencia del **Proyecto 2**. Toda
captura se organiza **por rubro de la rúbrica** (criterios del
`../AUDITORIA_RUBRICA.md`), siguiendo la misma convención que el Proyecto 1:
archivos `NN-descripcion.txt` (texto/consola) y `NN-descripcion.png`
(capturas de pantalla), con prefijo numérico de dos dígitos para el orden.

> **No se inventan resultados**: cada archivo debe salir de una ejecución real
> en la VM, con los comandos documentados en `../GUIA_DEMO.md`. Las
> carpetas solo se versionan con `.gitkeep`; las capturas las genera el
> estudiante en la VM (manual o con `scripts/capture-evidence.sh`) y se
> agregan a git al finalizar la entrega.

## Mapa de rubros → criterio → carpeta

| Rubro (carpeta) | Criterio de rúbrica | Qué contiene |
|---|---|---|
| `entorno/` | soporte transversal | `make doctor`, sistema operativo, versiones de herramientas, BTF/headers, `lsmod` del módulo, host de la VM |
| `kernel/` | 1.1, 1.2, 1.3, 2.1, 2.2 | compilación del módulo, carga/descarga, `/proc/continfo_pr2_so1_202300476` (JSON exacto + validación con `jq`), contraste con `ps`/`free` |
| `daemon-politica/` | 1.4, 1.5, 2.3 | `make build`, daemon en ejecución, interacción `/proc`→Valkey, política (conservar 3 low / 2 high), protegidos intactos |
| `cron-workloads/` | carga y cron del enunciado | `crontab -l` (bloque delimitado), generador instalado, 4 imágenes, generación de contenedores y acumulación de carga |
| `valkey-grafana/` | 1.5, 2.3, observabilidad | claves `so1:p2:*`, `system:latest`/timeline, rankings, salud de Grafana, capturas de **cada panel** del dashboard |
| `ebpf/` | 3.1, 3.2 | prueba privilegiada `sudo go test ./internal/ebpf/ -run Privilegiada`, stream `so1:p2:kills`, `kills:count > 0` con timestamps coherentes |
| `documentacion-git/` | 4.1, 4.2 y entrega Git | clon/checkout de la rama, `git log`, árbol de `docs/` y `docs/evidencias/`, capturas del repositorio en GitHub |

## Captura automática (texto)

`scripts/capture-evidence.sh` genera de forma **automática, idempotente y sin
secretos** los archivos de texto de cada rubro (entorno/doctor, estado del
módulo y `/proc` exacto validado con `jq`, contenedores/etiquetas, cron,
Valkey, salud de Grafana y eventos relevantes). Uso:

```bash
cd proyecto2
bash scripts/capture-evidence.sh                 # todo (solo lectura, sin sudo)
bash scripts/capture-evidence.sh --rubro kernel  # solo un rubro
bash scripts/capture-evidence.sh --privileged    # + lecturas con sudo (dmesg)
bash scripts/capture-evidence.sh --dry-run       # lista lo que capturaría, sin escribir
```

Cada corrida **sobrescribe** el archivo del rubro con un encabezado con la
fecha/hora, de modo que re-ejecutarlo no acumula basura. Los archivos
generados se llaman:

```
entorno/entorno.txt            kernel/kernel.txt
daemon-politica/politica.txt   cron-workloads/cron.txt
valkey-grafana/observabilidad.txt
ebpf/ebpf.txt                  documentacion-git/git.txt
```

## Capturas manuales (visuales)

El texto automático se complementa con **capturas de pantalla** tomadas a mano
(ver el guion de evidencias de `../GUIA_DEMO.md`):

- **Grafana**: una captura por cada uno de los 14 paneles del dashboard
  `deploy/grafana/dashboards/so1-p2-overview.json` (RAM total/usada/libre,
  evolución temporal, eliminaciones XLEN/GET/serie temporal, tabla eBPF,
  top 5 RAM/CPU, timeline de contenedores). Las capturas se guardan en
  `valkey-grafana/` con nombre `NN-grafana-<panel>.png`.
- **GitHub**: página del repositorio, pestaña *commits* de la rama
  `ricardious/p2-delivery` y *releases* (si aplica). Se guardan en
  `documentacion-git/`.

## Reglas de esta carpeta

1. No se versionan datos generados localmente en el desarrollo: solo el
   estudiante agrega las capturas reales de su VM antes de la entrega.
2. No se incluyen secretos: el `.env` local nunca se copia aquí; las capturas
   automáticas ya lo filtran.
3. Los `NN-` finales se renombran según el criterio de la rúbrica; si un archivo
   automático no aplica (p. ej. módulo no cargado), se elimina antes del
   commit.
