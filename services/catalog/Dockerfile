# Build context must be the repo root: docker build -f services/catalog/Dockerfile .
FROM golang:1.26-bookworm AS build
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY GOWORK=off
WORKDIR /src
COPY libs/ogc-kit/ libs/ogc-kit/
COPY services/catalog/ services/catalog/
RUN go build -C services/catalog -ldflags="-s -w" -o /out/catalog .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates \
    && rm -rf /var/lib/apt/lists/* && useradd -u 10001 geoson
USER geoson
COPY --from=build /out/catalog /usr/local/bin/catalog
ENV GEOSON_HTTP_ADDR=:8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD curl -fsS http://localhost:8080/healthz || exit 1
ENTRYPOINT ["catalog"]
