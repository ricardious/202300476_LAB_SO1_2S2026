# Módulo extra en Rust

Actividad adicional del Proyecto 2. Al cargar el módulo
`hola_rust_202300476.ko`, el kernel registra el mensaje:

```text
Hola Mundo 202300476
```

La lógica del módulo y la llamada a `_printk` están implementadas en
`hola_rust.rs`. `module.c` es un adaptador mínimo para registrar el ciclo de
vida `module_init/module_exit` con los headers distribuidos por Fedora.

## Requisitos

- Kernel compilado con `CONFIG_RUST=y`.
- `rustc` compatible con el kernel en ejecución.
- Headers `kernel-devel` correspondientes a `uname -r`.
- Privilegios de administrador para cargar y descargar el módulo.

## Uso

```bash
make                    # compilar hola_rust_202300476.ko
make load               # compilar y cargar
make verify             # mostrar el mensaje de carga en dmesg
make unload             # descargar
sudo dmesg | tail -n 10 # mostrar carga y descarga
```

Desde la raíz de `proyecto2` se dispone de los objetivos equivalentes:

```bash
make rust-module
make rust-module-load
make rust-module-verify
make rust-module-unload
```
