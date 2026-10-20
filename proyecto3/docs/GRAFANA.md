# Atlas: observatorio aéreo y marítimo

Atlas presenta los once indicadores del proyecto en tres áreas: panorama global, distribución por país y seguimiento de España. Utiliza los paneles nativos Stat, Bar gauge, Time series y Text de Grafana.

## Acceso

- [Dashboard](http://127.0.0.1:3003/d/so1-p3-202300476?theme=dark).
- [Vista de presentación](http://127.0.0.1:3003/d/so1-p3-202300476?theme=dark&kiosk), con controles de intervalo y actualización. Esc permite salir del modo kiosco.

El dashboard se configura como página de inicio y el tema predeterminado es oscuro. Una preferencia personal de tema puede sobrescribir el valor predeterminado; los enlaces anteriores seleccionan el tema previsto para Atlas.

## Lectura de los indicadores

El azul identifica aviones; el turquesa, barcos; y el ámbar destaca España. En los rankings, cada país conserva su propio color entre ambas clasificaciones. Las etiquetas permiten interpretar los datos sin depender del color.

Los extremos, las modas, los rankings y el total de reportes de España son acumulados del conjunto de datos. Cambiar el intervalo no recalcula esos agregados. Las modas conservan la lista de empates y muestran como número principal el menor valor modal. Los rankings suman unidades reportadas, no vehículos únicos.

Las series de España muestran el último estado por fecha del evento, observado cada cinco segundos. Se dibujan como escalones para evitar sugerir valores intermedios entre observaciones. Los ejes mantienen los límites del contrato: 0–50 aviones y 0–30 barcos. Un período sin nuevos reportes puede producir una línea horizontal.

## Mantenimiento

La fuente del dashboard está en `deploy/dashboard.json` y su copia para aprovisionamiento en `operator/helm-charts/grafanainstance/files/dashboard.json`. Ambas deben mantenerse idénticas. El operador incluye el chart en su imagen y reconcilia la configuración; los cambios deben publicarse mediante esa imagen. La anotación de checksum reinicia Grafana cuando cambia el aprovisionamiento.

La personalización utiliza HTML saneado en paneles Text, sin scripts ni hojas de estilo externas. Los gráficos mantienen componentes nativos, actualización cada diez segundos y el límite existente de 320 MiB de Grafana. No se incorporan servicios, fuentes remotas ni plugins de terceros.

La captura `evidencias/atlas-desktop.png` documenta el diseño con datos reales del entorno local.
