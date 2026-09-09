#!/usr/bin/env python3
"""Perfil 'intruder': intento controlado de lectura de archivos 'sensibles'.

Simula un atacante que intenta leer archivos sensibles, pero está acotado a
un fixture explícito montado dentro del contenedor en /opt/intruder/fixture.

REGLAS DE SEGURIDAD:
- NUNCA lee archivos reales del host fuera del fixture.
- No intenta escapar del contenedor ni acceder al namespace del host.
- Los 'intentos' sobre rutas del host se registran y se abortan sin abrir.

Sirve para demostrar en el dashboard el perfil de consumo y para que el
daemon pueda identificarlo por etiqueta como un workload gestionado.
"""

import glob
import os
import signal
import time

FIXTURE_DIR = "/opt/intruder/fixture"
HOST_PATHS = [
    "/etc/shadow",
    "/etc/passwd",
    "/root/.bash_history",
    "/proc/1/environ",
]

RUN_MINUTES = float(os.environ.get("SO1_RUN_MINUTES", "10"))


def _handler(_signum, _frame):
    raise SystemExit(0)


signal.signal(signal.SIGTERM, _handler)
signal.signal(signal.SIGINT, _handler)


def read_fixture(path):
    with open(path, "r", encoding="utf-8", errors="replace") as fh:
        return fh.read()


print("intruder: probing controlled fixture", flush=True)
for p in sorted(glob.glob(f"{FIXTURE_DIR}/*")):
    try:
        data = read_fixture(p)
        print(f"intruder: READ {os.path.basename(p)} -> {data!r}", flush=True)
    except OSError as exc:
        print(f"intruder: could not read {p}: {exc}", flush=True)

for host_path in HOST_PATHS:
    # Intento controlado: se registra y se aborta sin abrir el archivo real.
    print(f"intruder: blocked attempt on host path {host_path}", flush=True)

deadline = time.monotonic() + RUN_MINUTES * 60.0
while time.monotonic() < deadline:
    time.sleep(5)

print("intruder: finished nominal run", flush=True)
