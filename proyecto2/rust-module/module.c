// SPDX-License-Identifier: GPL-2.0

/*
 * Adaptador de ciclo de vida para el módulo escrito en Rust.
 * La lógica y los mensajes del módulo están en hola_rust.rs.
 */

#include <linux/init.h>
#include <linux/module.h>

extern int hola_rust_init(void);
extern void hola_rust_exit(void);

static int __init hola_rust_module_init(void)
{
	return hola_rust_init();
}

static void __exit hola_rust_module_exit(void)
{
	hola_rust_exit();
}

module_init(hola_rust_module_init);
module_exit(hola_rust_module_exit);

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Alex Ricardo Castaneda Rodriguez");
MODULE_DESCRIPTION("Modulo extra en Rust - SO1 Proyecto 2 - 202300476");
MODULE_VERSION("1.0.0");
