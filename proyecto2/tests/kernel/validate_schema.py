#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-2.0
"""
Validador del contrato JSON de /proc/continfo_pr2_so1_202300476
Carnet: 202300476 · Proyecto 2 SO1

Uso:
    python3 validate_schema.py <file.json> [--strict-memory]
    cat /proc/continfo_pr2_so1_202300476 | python3 validate_schema.py

Sin privilegios: funciona sobre cualquier documento que cumpla el esquema
(fixtures) y sobre la salida en vivo del módulo si este está cargado.

Contrato (PLAN_PROYECTO2.md, sección 3.1):
  {
    "schema_version": 1,
    "timestamp_unix_ns": 0,
    "memory": {"total_kb": 0, "free_kb": 0, "used_kb": 0},
    "processes": [
      {"pid":0,"name":"","cmdline":"","container_id":"",
       "vsz_kb":0,"rss_kb":0,"memory_percent":0.0,"cpu_percent":0.0}
    ]
  }
"""
import json
import re
import sys

PROC_FIELDS = {
    "pid": int,
    "name": str,
    "cmdline": str,
    "container_id": str,
    "vsz_kb": (int,),
    "rss_kb": (int,),
    "memory_percent": (int, float),
    "cpu_percent": (int, float),
}

MEM_FIELDS = {"total_kb": int, "free_kb": int, "used_kb": int}

HEX64 = re.compile(r"^[0-9a-fA-F]{64}$")


def check_memory(mem, strict=False):
    for k, t in MEM_FIELDS.items():
        if k not in mem:
            raise ValueError(f"memory: falta la clave '{k}'")
        if not isinstance(mem[k], t) or mem[k] < 0:
            raise ValueError(f"memory.{k}: debe ser entero >= 0")
    total = mem["total_kb"]
    if strict:
        # used = total - free según la definición del módulo.
        if total > 0 and mem["used_kb"] != total - mem["free_kb"]:
            raise ValueError(
                f"memory: used_kb ({mem['used_kb']}) != total - free "
                f"({total} - {mem['free_kb']})"
            )


def check_process(p):
    for k, t in PROC_FIELDS.items():
        if k not in p:
            raise ValueError(f"proceso: falta la clave '{k}'")
        if not isinstance(p[k], t):
            raise ValueError(f"proceso.{k}: tipo incorrecto ({type(p[k]).__name__})")
    if p["pid"] <= 0:
        raise ValueError(f"proceso: pid debe ser > 0 (visto: {p['pid']})")
    if p["vsz_kb"] < 0 or p["rss_kb"] < 0:
        raise ValueError(f"proceso {p['pid']}: vsz_kb/rss_kb deben ser >= 0")
    if p["rss_kb"] > p["vsz_kb"]:
        raise ValueError(f"proceso {p['pid']}: rss_kb ({p['rss_kb']}) > vsz_kb ({p['vsz_kb']})")
    if p["memory_percent"] < 0 or p["cpu_percent"] < 0:
        raise ValueError(f"proceso {p['pid']}: porcentajes deben ser >= 0")
    cid = p["container_id"]
    if cid and not re.match(r"^[0-9a-fA-F]{32,64}$", cid):
        raise ValueError(f"proceso {p['pid']}: container_id '{cid}' no es un id plausible")


def main():
    strict = "--strict-memory" in sys.argv
    if len(sys.argv) >= 2 and sys.argv[1] != "--strict-memory":
        with open(sys.argv[1], "r", encoding="utf-8") as f:
            raw = f.read()
    else:
        raw = sys.stdin.read()

    try:
        doc = json.loads(raw)
    except json.JSONDecodeError as e:
        print(f"FAIL: JSON inválido: {e}", file=sys.stderr)
        sys.exit(1)

    # claves top-level
    for k in ("schema_version", "timestamp_unix_ns", "memory", "processes"):
        if k not in doc:
            print(f"FAIL: falta la clave raíz '{k}'", file=sys.stderr)
            sys.exit(1)
    if doc["schema_version"] != 1:
        print(f"FAIL: schema_version debe ser 1 (visto: {doc['schema_version']})", file=sys.stderr)
        sys.exit(1)
    if not isinstance(doc["timestamp_unix_ns"], int) or doc["timestamp_unix_ns"] < 0:
        print("FAIL: timestamp_unix_ns debe ser entero >= 0", file=sys.stderr)
        sys.exit(1)
    if not isinstance(doc["processes"], list):
        print("FAIL: processes debe ser una lista", file=sys.stderr)
        sys.exit(1)

    check_memory(doc["memory"], strict=strict)

    for p in doc["processes"]:
        check_process(p)

    # pids únicos
    pids = [p["pid"] for p in doc["processes"]]
    if len(pids) != len(set(pids)):
        print("FAIL: hay pids duplicados", file=sys.stderr)
        sys.exit(1)

    print(f"OK: JSON válido · {len(doc['processes'])} procesos · "
          f"mem total={doc['memory']['total_kb']} KB")


if __name__ == "__main__":
    main()