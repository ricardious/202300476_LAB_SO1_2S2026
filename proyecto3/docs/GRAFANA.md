# Consola de operaciones M.U.M.N.

La consola aérea y marítima presenta el mapa de los cinco países, el último estado del país seleccionado, los agregados globales y los rankings. El panel propio `so1-operations-panel` se ejecuta dentro de Grafana. Las dos series inferiores muestran el historial de España, el país asignado al proyecto 202300476.

## Acceso y navegación

- [Consola](http://127.0.0.1:3003/d/so1-p3-202300476?theme=dark&kiosk).
- [Dashboard con navegación de Grafana](http://127.0.0.1:3003/d/so1-p3-202300476?theme=dark).

Seleccionar una pestaña, un marcador del mapa o una fila del ranking actualiza la ficha lateral y el indicador de selección. Los marcadores también se pueden activar con Tab y Enter o espacio. La selección inicial es ESP; cambia únicamente la ficha del país y el foco del mapa. Las estadísticas globales y las series inferiores de España conservan su alcance. En pantallas pequeñas la composición se apila y el panel admite desplazamiento vertical.

El azul identifica aviones, el cian barcos y el ámbar la selección. Las etiquetas permiten interpretar la consola sin depender del color. Los marcadores son referencias geográficas fijas por país: los reportes no contienen coordenadas de vehículos ni rutas. Los círculos del mapa indican el país seleccionado, no un área de cobertura o detección.

## Indicadores y significado

| Indicador del proyecto | Ubicación |
| --- | --- |
| Máximo, mínimo y moda de aviones | Banda de instrumentos: Aviones |
| Máximo, mínimo y moda de barcos | Banda de instrumentos: Barcos |
| Top 5 de aviones y barcos | Rankings inferiores de la consola |
| Reportes de España | Ficha lateral con ESP seleccionado al abrir |
| Evolución de aviones y barcos de España | Dos series temporales bajo la consola |

Los extremos, las modas, los rankings y el número de reportes son acumulados históricos. Cambiar el intervalo no recalcula esos agregados. La moda muestra el menor valor modal como número principal y todos los empates en «Valores». Los rankings suman unidades reportadas, no vehículos únicos; los empates se ordenan por código de país.

La ficha lateral muestra el último estado por fecha del evento. «Datos consultados» indica que la consulta devolvió datos, no que llegaron nuevos reportes ni que todos los servicios estén saludables. Los ceros son mediciones válidas; los valores ausentes se muestran como un guion. Ante un error de consulta se muestra una advertencia y se ocultan las mediciones retenidas.

Las series de España muestran estados observados cada cinco segundos con líneas escalonadas. Los ejes mantienen los límites del contrato: 0–50 aviones y 0–30 barcos. Un período sin reportes nuevos puede producir una línea horizontal. La retención local es de dos horas y la actualización del dashboard es cada diez segundos.

## Implementación y despliegue

El plugin se mantiene en `operator/helm-charts/grafanainstance/files/console/`: `module.js`, `plugin.json`, `world.svg` y `logo.svg`. Es un módulo SystemJS legible que reutiliza React y `PanelPlugin` provistos por Grafana; no requiere un compilador, paquetes npm ni un backend adicional. Los estilos del panel están limitados a `.so1-console`. En modo kiosco se ajusta además el contenedor de desplazamiento de Grafana y su pie de página, únicamente cuando el dashboard contiene esta consola. Esto evita que el pie se superponga a la cuadrícula en ventanas bajas; se conserva la marca de Grafana. El ajuste usa el atributo `data-testid="public-dashboard-footer"` de Grafana 13 y debe revisarse al actualizar Grafana. No hay temporizadores de animación, tipografías remotas ni peticiones cartográficas externas.

El chart monta los archivos de solo lectura mediante un ConfigMap. `GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS` permite exclusivamente el ID del plugin local `so1-operations-panel`; no se desactiva el saneamiento de HTML. Grafana registra una advertencia esperada de plugin sin firma de catálogo. Su compatibilidad declarada se limita a Grafana 13.2.1–13.x; la versión comprobada es 13.2.1.

`deploy/dashboard.json` y `operator/helm-charts/grafanainstance/files/dashboard.json` deben permanecer idénticos. La imagen del operador incluye el chart y sus archivos. Al publicar cambios, el checksum de aprovisionamiento reinicia Grafana para cargar el nuevo módulo. Los cambios se conservan al reiniciar el entorno.

El límite de Grafana continúa en 320 MiB. El tamaño estático del plugin no equivale a su uso de memoria; el consumo real depende también de Grafana y del navegador.

## Validación

`make -C proyecto3 test-console` ejecuta pruebas del lector de datos, ceros, valores ausentes, empates, orden de rankings y estados de error. Requiere Node.js con `node:test` (Node 20 o posterior); Node no se necesita en el despliegue. La validación visual se realiza en Grafana con datos reales, en escritorio y móvil, incluyendo selección por pestaña, mapa y teclado.

La evidencia actual es `evidencias/console-desktop.png`. La captura anterior `atlas-desktop.png` conserva el diseño previo.

## Referencias

- [Paneles personalizados de Grafana](https://grafana.com/developers/plugin-tools/tutorials/build-a-panel-plugin).
- [Firmas de plugins](https://grafana.com/docs/grafana/latest/administration/plugin-management/plugin-sign/).
- Mapa derivado de [Natural Earth, tierra a escala 1:110m](https://github.com/nvkelso/natural-earth-vector/blob/master/geojson/ne_110m_land.geojson), de [dominio público](https://www.naturalearthdata.com/about/terms-of-use/). Proyección equirectangular: x = (longitud + 180) × 2.5; y = (85 − latitud) × 3. El recorte excluye la Antártida. Se conserva localmente como SVG.
