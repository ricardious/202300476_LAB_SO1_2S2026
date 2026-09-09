# ============================================================================
# Dockerfile - API1 (Proyecto 1 SO1 · Carnet 202300476)
#
# Imagen final construida como: api1-202300476
#   (minúsculas por restricción de Docker/Podman: los nombres de repositorio
#    de imagen deben ser minúsculas)
# Nombre canónico solicitado por el enunciado: API1-202300476
#   -> se conserva como LABEL dentro de la imagen.
#
# Build reproducible en 2 etapas e imagen final pequeña.
# ============================================================================

# ---- Etapa 1: compilación (solo dependencias necesarias) ----
FROM golang:1.21-alpine AS builder

WORKDIR /src

# Copiamos go.mod para cachear dependencias de forma reproducible
COPY go.mod ./
RUN go mod download

# Código fuente de la API
COPY main.go ./

# Binario estático (sin CGO) y mínimo
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /api1 .

# ---- Etapa 2: runtime mínimo ----
FROM alpine:3.20

LABEL org.opencontainers.image.title="API1-202300476" \
      org.opencontainers.image.description="API1 del Proyecto 1 SO1 (carnet 202300476)" \
      org.opencontainers.image.version="1.0"

# Certificados para llamadas HTTP(S) salientes
RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /api1 /app/api1

# Puerto de escucha de API1 (vive en VM1)
EXPOSE 8081

# Usuario no privilegiado (no se necesitan herramientas adicionales en la imagen)
USER 65534:65534

ENTRYPOINT ["/app/api1"]
