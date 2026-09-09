#!/usr/bin/env python3
"""Perfil 'cpu-high': carga alta de CPU con límites configurables.

Consume CPU de forma sostenida (cómputo en lazo cerrado) durante toda la
ejecución. El tope real lo impone Docker con --cpus (por defecto 0.5 en el
generador), de modo que la carga queda acotada aunque el bucle intente
usar todos los núcleos disponibles.

Acepta SO1_CPU_LOAD (0-100, default 100) y SO1_RUN_MINUTES (default 10).
"""

import os
import signal
import time

LOAD = max(0.0, min(100.0, float(os.environ.get("SO1_CPU_LOAD", "100"))))
RUN_MINUTES = float(os.environ.get("SO1_RUN_MINUTES", "10"))


def _handler(_signum, _frame):
    raise SystemExit(0)


signal.signal(signal.SIGTERM, _handler)
signal.signal(signal.SIGINT, _handler)

print(
    f"cpu-high: burning CPU at ~{LOAD:.0f}% for {RUN_MINUTES:.0f} min "
    f"(bounded by docker --cpus)",
    flush=True,
)

deadline = time.monotonic() + RUN_MINUTES * 60.0
work_period = LOAD / 100.0
idle_period = 1.0 - work_period

while time.monotonic() < deadline:
    if LOAD >= 99.9:
        # Lazo cerrado: máxima CPU posible dentro del límite de cpus.
        for _ in range(4_000_000):
            pass
    else:
        busy_until = time.monotonic() + work_period * 0.1
        while time.monotonic() < busy_until:
            pass
        time.sleep(idle_period * 0.1)

print("cpu-high: finished nominal run", flush=True)
