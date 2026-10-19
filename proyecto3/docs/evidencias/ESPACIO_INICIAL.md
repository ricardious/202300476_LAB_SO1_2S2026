# Preparación del almacenamiento

Inspección inicial: raíz 3.6 GiB libres, desarrollo 1.1 GiB libres y partición adicional 16 GiB libres.

Limpieza realizada con los gestores npm y pnpm y retirada del almacén pnpm v10 regenerable. Se conservan los datos de Oracle, PostgreSQL, Mosquitto y Proyecto 2. La caché de Orca contiene el runtime activo y se conserva. No se borran Steam, PlatformIO ni código.

Después de esta limpieza: raíz aproximadamente 3.9 GiB y desarrollo 2.3 GiB libres. Los objetivos de 10 y 6 GiB todavía no se alcanzaron. Los datos nuevos del Proyecto 3 se guardan en `/mnt/fedora-extra/so1-proyecto3`; no se trasladan archivos personales para simular espacio recuperado.

Se retiraron además las cachés ignoradas por Git `client/.next/cache` y `client/.next/dev/cache` del entorno de desarrollo inactivo de Algemify, conservando sus cambios de código, dependencias y resultados de compilación. Espacio adicional recuperado: 1467 MiB.

Estado final medido después de instalar y probar: raíz 3.6 GiB, desarrollo 3.8 GiB y partición adicional 8.2 GiB libres. Se recuperaron aproximadamente 3 GiB de cachés previas; las instalaciones nuevas compensan parte de esa ganancia. Se preservan todos los archivos personales y las bases de datos.
