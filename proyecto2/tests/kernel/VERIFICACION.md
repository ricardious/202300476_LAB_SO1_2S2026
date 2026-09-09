# Verificación del frente kernel — Proyecto 2 SO1 (202300476)

Resultado de las verificaciones ejecutadas para el módulo de kernel
`/proc/continfo_pr2_so1_202300476`. Documenta la evidencia de compilación y
las **pruebas privilegiadas pendientes** que solo pueden ejecutarse con root
en la VM objetivo.

## 1. Evidencia de compilación (ejecutada y exitosa)

El módulo se compiló contra los headers EXACTOS del kernel en ejecución.
La máquina de desarrollo no tenía `kernel-devel` instalado, por lo que la
compilación se realizó en un contenedor Docker (Fedora 42) con
`kernel-devel-6.19.14-108.fc42.x86_64`, los headers del mismo kernel que
corre en esta máquina.

- Kernel en ejecución (host): `6.19.14-108.fc42.x86_64`
- Headers usados: `/usr/src/kernels/6.19.14-108.fc42.x86_64`
- Comando:
  `make -C /usr/src/kernels/6.19.14-108.fc42.x86_64 M=/build/kernel modules`
- Salida (exit 0):

```
  CC [M]  continfo.o
  MODPOST Module.symvers
  CC [M]  continfo.mod.o
  CC [M]  .module-common.o
  LD [M]  continfo.ko
  BTF [M] continfo.ko
Skipping BTF generation for continfo.ko due to unavailability of vmlinux
```

- Cero warnings del código del módulo (las únicas advertencias provienen de
  cabeceras del propio kernel, `asm/ibt.h`, y aparecen en cualquier módulo
  compilado con esta cadena de herramientas; no son del módulo).
- `modinfo` del `.ko` generado:
  - `vermagic=6.19.14-108.fc42.x86_64 SMP preempt mod_unload` (coincide con
    el kernel en ejecución)
  - `depends=`
- Interfaz esperada tras `insmod`: `/proc/continfo_pr2_so1_202300476`

> Nota de portabilidad: `vermagic` exige compilar contra los headers del
> kernel destino. En la VM objetivo basta con `make -C proyecto2/kernel`
> (o `proyecto2/scripts/load-module.sh`, que compila si falta el `.ko`).

## 2. Verificaciones estáticas / sin privilegios (ejecutadas)

| Prueba | Resultado |
|---|---|
| `test-schema.sh` (fixture `fixtures/sample-snapshot.json`) | OK — JSON válido |
| `test-build.sh` (compila si hay headers) | SKIP — sin `kernel-devel` en el host (la compilación real se hizo en el contenedor, ver §1) |
| `test-module-lifecycle.sh` | SKIP — requiere root (prueba privilegiada pendiente) |
| `test-all.sh` | OK (con SKIP documentados) |
| `bash -n` (sintaxis) de scripts y tests | OK |
| `python3 -m py_compile validate_schema.py` | OK |
| `shellcheck` | no disponible en el entorno (se omitió) |

En la máquina de desarrollo los headers del kernel NO están instalados
(`/lib/modules/6.19.14-108.fc42.x86_64/build` es un enlace roto y no hay
permisos de instalación). Por eso `test-build.sh` omite la compilación
local; la compilación equivalente se demostró en el contenedor (§1).

## 3. Pruebas privilegiadas PENDIENTES (requieren la VM objetivo)

Se requiere root en la VM objetivo (máquina Linux donde se cargará el
módulo). En la máquina de desarrollo el intento de carga desde un
contenedor privilegiado fue denegado por la política de seguridad del host
(SELinux en modo enforcing + política de contenedores), por lo que la carga
real queda pendiente. Procedimiento en la VM:

```sh
# 1) headers del kernel destino (idéntico al kernel que correrá)
sudo dnf install kernel-devel-$(uname -r)   # Fedora/RHEL
sudo apt install linux-headers-$(uname -r)  # Debian/Ubuntu

# 2) compilar
make -C proyecto2/kernel

# 3) ciclo de vida completo (carga idempotente, lectura, JSON, memoria,
#    CPU de un proceso acotado, descarga idempotente, eliminación de /proc)
sudo bash proyecto2/tests/kernel/test-module-lifecycle.sh

# 4) validar contrato JSON contra la salida en vivo
cat /proc/continfo_pr2_so1_202300476 | python3 proyecto2/tests/kernel/validate_schema.py --strict-memory

# 5) carga/descarga idempotentes manuales
sudo bash proyecto2/scripts/load-module.sh     # dos veces -> OK la segunda
sudo bash proyecto2/scripts/unload-module.sh   # dos veces -> OK la segunda
```

Verificaciones que cubre `test-module-lifecycle.sh` (rúbrica 1.2, 1.3, 2.1,
2.2):

1. Idempotencia de `load-module.sh` / `unload-module.sh`.
2. Creación y eliminación de `/proc/continfo_pr2_so1_202300476`.
3. JSON válido contra el esquema (validado además con `--strict-memory`).
4. `memory.total_kb` vs `MemTotal` de `/proc/meminfo` (tolerancia ≤ 1 %);
   `free_kb` vs `MemFree` (tolerancia ≤ 512 MB por la variación temporal).
5. PID 1 (`systemd`) presente y shell actual presente con `cmdline`.
6. `cpu_percent` de un proceso con carga acotada (spin ~2 s) debe ser > 5 %.

## 4. Contrato JSON (resumen)

```json
{
  "schema_version": 1,
  "timestamp_unix_ns": 1720000000123456789,
  "memory": { "total_kb": 0, "free_kb": 0, "used_kb": 0 },
  "processes": [
    { "pid": 0, "name": "", "cmdline": "", "container_id": "",
      "vsz_kb": 0, "rss_kb": 0, "memory_percent": 0.0, "cpu_percent": 0.0 }
  ]
}
```

Decisiones documentadas (ver `kernel/continfo.c` para los detalles):

- Memoria: `si_meminfo()` → `totalram_pages()`/`nr_free_pages()`, igual que
  `MemTotal`/`MemFree`; `used_kb = total_kb - free_kb`.
- `vsz_kb = mm->total_vm << (PAGE_SHIFT - 10)`; `rss_kb = get_mm_rss(mm) << ...`.
- `memory_percent = rss_kb / total_kb * 100` con dos decimales.
- `cpu_percent = (utime+stime) / (ahora - start_time) * 100` — promedio sobre
  la vida del proceso, misma convención que `ps`; `utime/stime`/`start_time`
  están en nanosegundos en kernels modernos (verificado contra los headers).
- `container_id`: se lee `/proc/<pid>/cgroup` y se extrae el token hexadecimal
  más largo (≥ 32 chars; Docker usa 64). Vacío si no parece contenedor;
  `cmdline` queda como alternativa verificable.
- Referencias: `get_task_struct()` bajo RCU y `get_task_mm()`/`mmput()`;
  el formateo se realiza FUERA de los bloqueos de tareas.