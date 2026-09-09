# Imagen reproducible de Grafana para el Proyecto 2.
#
# Fija la versión base de Grafana y la versión del plugin Redis/Valkey para
# que el datasource provisionado funcione siempre igual, sin pasos manuales.
ARG GRAFANA_VERSION=13.2.1
FROM grafana/grafana:${GRAFANA_VERSION}

ARG GRAFANA_VERSION=13.2.1
ARG REDIS_PLUGIN_VERSION=2.2.0

# Instala el datasource Redis/Valkey con versión fijada.
# Grafana 13 ya no trae `grafana-cli`; la gestión de plugins es `grafana cli`.
RUN /usr/share/grafana/bin/grafana cli plugins install redis-datasource ${REDIS_PLUGIN_VERSION}

# Provisioning: datasources, dashboard provider y dashboard.
COPY provisioning/datasources /etc/grafana/provisioning/datasources
COPY provisioning/dashboards /etc/grafana/provisioning/dashboards
COPY dashboards /var/lib/grafana/dashboards

ENV GF_VERSION=${GRAFANA_VERSION}