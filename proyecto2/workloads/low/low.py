#!/usr/bin/env python3
"""Perfil 'low': consumo mínimo durante al menos cuatro minutos.

Mantiene un uso de CPU y RAM prácticamente nulo mientras el proceso sigue
vivo, de modo que el daemon de telemetría pueda identificarlo como un
contenedor de bajo consumo (nunca candidato a eliminación por debajo del
mínimo de 3).

Acepta SO1_RUN_MINUTES (default 10) para controlar la duración en pruebas.
"""

import os
import signal
import time

RUN_MINUTES = float(os.environ.get("SO1_RUN_MINUTES", "10"))


def _handler(_signum, _frame):
    raise SystemExit(0)


signal.signal(signal.SIGTERM, _handler)
signal.signal(signal.SIGINT, _handler)

deadline = time.monotonic() + RUN_MINUTES * 60.0
_keep_alive = bytearray(4 * 1024 * 1024)

print(f"low: running for {RUN_MINUTES:.0f} minutes with minimal footprint", flush=True)

while True:
    if time.monotonic() >= deadline:
        break
    _keep_alive[0] = (_keep_alive[0] + 1) & 0xFF
    time.sleep(5)

print("low: finished nominal run", flush=True)
