#!/usr/bin/env python3
"""Perfil 'memory-high': consumo alto y acotado de RAM.

Reserva progresivamente bloques de memoria hasta alcanzar el tope indicado
por SO1_MEM_TARGET_MB (default 512 MiB) y mantiene la reserva durante toda
la ejecución. No supera el límite de memoria fijado por Docker (que es el
que realmente protege a la VM), así que el consumo se mantiene acotado.

La reserva se escribe (no solo se asigna con malloc) para que cuente como
RSS real ante el módulo de kernel y Docker stats.
"""

import os
import signal
import time

TARGET_MB = float(os.environ.get("SO1_MEM_TARGET_MB", "512"))
CHUNK_MB = 4
RUN_MINUTES = float(os.environ.get("SO1_RUN_MINUTES", "10"))

# 1 MiB de bytes, tocado para forzar RSS real.
CHUNK = bytearray(CHUNK_MB * 1024 * 1024)


def _handler(_signum, _frame):
    raise SystemExit(0)


signal.signal(signal.SIGTERM, _handler)
signal.signal(signal.SIGINT, _handler)

blocks = []
try:
    while (len(blocks) + 1) * CHUNK_MB <= TARGET_MB:
        blocks.append(bytearray(CHUNK))
except MemoryError:
    pass

reserved_mb = len(blocks) * CHUNK_MB
print(
    f"memory-high: reserved ~{reserved_mb} MiB "
    f"(target {TARGET_MB:.0f} MiB), holding for {RUN_MINUTES:.0f} min",
    flush=True,
)

deadline = time.monotonic() + RUN_MINUTES * 60.0
while time.monotonic() < deadline:
    if blocks:
        blocks[0][0] = (blocks[0][0] + 1) & 0xFF
    time.sleep(1)

print("memory-high: finished nominal run", flush=True)
