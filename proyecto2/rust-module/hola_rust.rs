// SPDX-License-Identifier: GPL-2.0

//! Lógica del módulo extra de Rust para Linux.

#![no_std]
#![no_main]

use core::ffi::{c_char, c_int};
use core::panic::PanicInfo;

unsafe extern "C" {
    fn _printk(format: *const c_char, ...) -> c_int;
}

const LOAD_MESSAGE: &[u8] = b"<6>Hola Mundo 202300476\n\0";
const UNLOAD_MESSAGE: &[u8] = b"<6>Modulo Rust 202300476 descargado\n\0";

#[unsafe(no_mangle)]
pub extern "C" fn hola_rust_init() -> c_int {
    // SAFETY: el formato es una cadena C estática, terminada en NUL y sin
    // especificadores; por tanto no requiere argumentos variádicos.
    unsafe {
        _printk(LOAD_MESSAGE.as_ptr().cast());
    }
    0
}

#[unsafe(no_mangle)]
pub extern "C" fn hola_rust_exit() {
    // SAFETY: misma justificación que en `hola_rust_init`.
    unsafe {
        _printk(UNLOAD_MESSAGE.as_ptr().cast());
    }
}

#[panic_handler]
fn panic(_info: &PanicInfo<'_>) -> ! {
    loop {
        core::hint::spin_loop();
    }
}
