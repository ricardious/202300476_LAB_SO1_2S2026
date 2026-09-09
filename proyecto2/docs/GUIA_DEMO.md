# Guía de Demostración — Proyecto 2 SO1 (202300476)

Runbook de la demo de punta a punta con los comandos exactos y la salida
esperada. Sirve también como guion para capturar evidencias.

---

## 0. Estado ideal previo

```bash
cd <carpeta>/proyecto2
sudo make install
make up                       # Valkey + Grafana
sudo make module-load
make install-cron             # o lo instala el daemon al arrancar
```

## 1. Demo automática (pipeline)

```bash
bash scripts/demo.sh                          # pasos 1-3 (Compose + 5 contenedores + URL)
sudo bash scripts/demo.sh --daemon            # + daemon real (módulo + eBPF con root)
```

Salida esperada:

```
[1/4] levantando Valkey + Grafana (Compose)...
[2/4] generando 5 contenedores de perfil aleatorio...
NOMBRE  IMAGEN  ESTADO
so1-load-memory-high-1720000000-1234-0  so1/load:202300476-memory-high  Up ...
...
[3/4] dashboard en http://localhost:3000 (admin/admin)
[4/4] arrancando daemon ...
```

## 2. Verificación manual del módulo

```bash
cat /proc/continfo_pr2_so1_202300476 | python3 -m json.tool | head -40
sudo dmesg | tail -5                 # mensajes de carga del módulo
python3 tests/kernel/validate_schema.py /proc/continfo_pr2_so1_202300476 --strict-memory
```

## 3. Acumular carga para provocar eliminaciones

El daemon solo elimina cuando hay **más de 5 contenedores gestionados
corriendo** (3 low + 2 high + excedentes). El cron crea 5 cada 2 min y los
contenedores viven ~10 min, así que en ~12 min hay excedentes. Para acelerar:

```bash
bash scripts/generate-containers.sh   # +5
bash scripts/generate-containers.sh   # +5 (total 10)
docker ps --filter label=so1.p2.managed=1 --format '{{.Names}}'
```

Con 10 contenedores, el daemon (loop 20 s) debe conservar 5 y solicitar la
terminación de 5. Cada terminación genera un evento eBPF que, correlacionado,
incrementa `so1:p2:kills:count`.

## 4. Verificación de la política (conservar 3 low + 2 high, nunca Grafana)

```bash
# Contadores por tier (aproximado por consumo) y supervivientes:
docker ps --filter label=so1.p2.managed=1 --format '{{.Names}} {{.Image}}'
docker ps --format '{{.Names}}' | grep so1-p2-grafana   # siempre presente
```

## 5. Cadena de confirmación eBPF → Valkey

```bash
docker exec so1-p2-valkey valkey-cli XLEN so1:p2:kills
docker exec so1-p2-valkey valkey-cli GET so1:p2:kills:count
docker exec so1-p2-valkey valkey-cli XRANGE so1:p2:kills - + | head
docker exec so1-p2-valkey valkey-cli XRANGE so1:p2:system:timeline - + | head
```

`kills:count` debe ser **mayor que 0** solo después de que el daemon solicitó
terminaciones y eBPF capturó el `kill` del contenedor.

## 6. Dashboard

Abrir `http://localhost:3000` (admin/admin). Verificar paneles:

1. **RAM total / usada / libre** (cards) — valores de `so1:p2:system:latest`.
2. **Evolución temporal de RAM** — serie de `mem_used_kb`.
3. **Eliminaciones confirmadas (XLEN)** y **contador total (GET)**.
4. **Eliminaciones a lo largo del tiempo** — puntos/barra por evento.
5. **Tabla de eventos kill capturados por eBPF** — últimos 10 eventos.
6. **Top 5 histórico por RAM/CPU** — miembros `"<id>|pid=<pid>"`.
7. **Timeline de contenedores** — activos e inactivos.

## 7. Idempotencia y apagado

```bash
bash scripts/generate-containers.sh    # otra corrida: no falla, nombres únicos
crontab -l                             # una sola sección del proyecto
kill -TERM <pid-del-daemon>            # apagado ordenado
crontab -l                             # la sección del proyecto desapareció
```

## 8. Evidencias a capturar (guion)

| # | Evidencia | Comando / acción |
|---|---|---|
| 1 | Compilación del módulo | `sudo make module` |
| 2 | Carga/descarga | `sudo make module-load` + `ls /proc/continfo_*` + `sudo make module-unload` |
| 3 | JSON válido de `/proc` | `cat /proc/... \| python3 -m json.tool` + `validate_schema.py` |
| 4 | Daemon compila y corre | `make build` + logs del daemon |
| 5 | Interacción `/proc`+Valkey | `valkey-cli HGETALL so1:p2:system:latest` + timeline |
| 6 | eBPF captura kill | `sudo go test ./internal/ebpf/ -run Privilegiada -v -count=1` |
| 7 | Correlación eBPF→Valkey | `valkey-cli XRANGE so1:p2:kills - +` con `kills:count > 0` |
| 8 | Cron 5 contenedores/2 min | `crontab -l` + `docker ps \| wc -l` a lo largo del tiempo |
| 9 | Dashboard con datos | captura de cada panel (screenshots) |
| 10 | Conservación 3/2 y Grafana vivo | `docker ps` antes/después de la eliminación |
| 11 | Limpieza segura | `bash scripts/cleanup.sh --no-confirm` (Proyecto 1 intacto) |