# Guía de instalación local

## Equipo y prerrequisitos

Perfil validado en Fedora 42 x86_64, Ryzen 5 7520U, 7 GiB de RAM utilizables, Docker 29 y Podman rootless. Se necesitan Python 3 con `cryptography`, `bcrypt`, `PyYAML` y `venv`, Go 1.25 o posterior, Rust compatible con el archivo Cargo.lock y un compilador C. El usuario debe tener acceso funcional a Docker y escritura en la partición adicional.

El equipo tenía inicialmente 1.1 GiB libres en desarrollo y 3.6 GiB en el sistema. Se retiraron cachés npm y pnpm y una caché pequeña de PlatformIO; se conservaron bases de datos, aplicaciones y código. Se retiraron también 1467 MiB de cachés regenerables de Next.js de un entorno inactivo, conservando sus cambios de código y dependencias. La meta inicial de liberar 10 GiB en raíz y 6 GiB en desarrollo no se alcanzó con esas cachés. El proyecto mantiene sus datos fuera de ambas particiones. Cerrar aplicaciones pesadas antes de usar todo el entorno.

`make bootstrap` descarga binarios x86_64 en el directorio de datos, valida sus SHA256 contra `scripts/tools.lock.json` y crea un único clúster exclusivo `so1-p3`. No instala herramientas globales ni modifica el kubeconfig personal. El SDK se utilizó para generar el operador; no se necesita compilar un controlador Go propio.

## Pasos iniciales

```bash
cd proyecto3
cp .env.example .env
make bootstrap
make setup-load
make mirror
make build
make up
make test
make e2e
make dapr-test
make load USERS=5 DURATION=30s
```

La primera instalación necesita Internet para paquetes, imágenes y herramientas. No usa Google Cloud ni activa facturación. Los nombres de clúster y namespace están fijados en `so1-p3` para mantener coherentes los manifiestos. El directorio de datos y los puertos de acceso pueden configurarse mediante `.env`.

La construcción se realiza con cachés externas, un job de Cargo y un paquete Go simultáneo. Las imágenes se construyen en formato OCI y se publican con HTTPS. El mirror copia una sola arquitectura y convierte únicamente los manifiestos Docker v2 cuando Zot los rechaza, conservando y validando los digests de los blobs.

## Certificados y credenciales

Zot usa puerto 5005 y una CA local. El certificado contempla `localhost`, `127.0.0.1` y `host.k3d.internal`. Importar el archivo `/mnt/fedora-extra/so1-proyecto3/certs/ca.crt` en las autoridades del navegador para abrir el catálogo sin advertencias. Se requiere autenticación HTTP Basic con el usuario `so1`.

```bash
make credentials
```

Las credenciales y claves quedan en `runtime/` y `certs/` del directorio externo; los archivos sensibles tienen permisos 0600. No subirlos a Git ni incluir la salida de `make credentials` en evidencias. Mantener Zot accesible al bridge Docker local; el script no abre puertos del firewall hacia Internet.

## Accesos

`make up` abre dos túneles en localhost: Gateway en 8088 y Grafana en 3003. `make up` también deja preparada la interfaz de Locust en 8089, sin iniciar tráfico. `make load-ui` permite abrirla por separado. Empezar con 1–5 usuarios y aumentar gradualmente.

El perfil de carga se publica como `localhost:5005/so1/input:1.0.0` y se descarga antes de ejecutar Locust con `make load` o `make load-ui`. El archivo descargado se compara con el original. La semilla y los rangos se encuentran en `load/load-profile.json`.

El flujo extra Dapr queda disponible en `/dapr-202300476`. `make up` habilita su pod con dos contenedores. `make dapr-off` permite pausarlo conservando el flujo principal; `make dapr-on` lo reactiva. No requiere instalar el CLI ni el plano de control de Dapr. Ver [configuración, credenciales y pruebas](DAPR.md).

## Diagnóstico

```bash
make doctor
make status
source scripts/env.sh
k get events --sort-by=.lastTimestamp
k logs deploy/api-rust --tail=10
k logs deploy/consumer --tail=10
k logs deploy/p3-operator --tail=20
k logs daemonset/falco --tail=20
```

Un servicio recién instalado puede tardar en estar listo; Grafana realiza migraciones y Falco carga programas eBPF. Readiness por sí sola no prueba que Falco esté capturando: generar y verificar la alerta.

El disco raíz casi lleno activó inicialmente `DiskPressure`. El perfil de kubelet conserva un umbral absoluto de 2 GiB para el filesystem del nodo y de imágenes, mantiene reclamación mínima de 512 MiB y coloca logs bajo el directorio externo de K3s. No elimina la protección de disco. Si raíz cae por debajo de ese margen, liberar espacio antes de continuar.

Si se reinicia el contenedor del nodo directamente con Docker, se puede perder la entrada `host.k3d.internal`. Usar `make bootstrap` para restaurar el arranque y la resolución del registro. Si se reemplaza Grafana o Envoy, ejecutar `make tunnels` para renovar su port-forward.

## Parada y conservación

```bash
make down
```

Detiene únicamente `so1-p3`, sus túneles registrados, la interfaz Locust registrada y el proceso Zot asociado al directorio del proyecto. Conserva PVC, datos, imágenes y credenciales. No ejecuta `docker system prune`, no vacía volúmenes y no detiene PostgreSQL ni Mosquitto de otros proyectos.

No borrar ni mover el directorio externo mientras el clúster esté encendido. La retirada definitiva de los datos es una operación separada y deliberada; no forma parte de la parada diaria.
