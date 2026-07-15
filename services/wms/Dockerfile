# Build context must be the repo root: docker build -f services/wms/Dockerfile .
FROM rust:1-alpine AS build
RUN apk add --no-cache musl-dev
WORKDIR /src
COPY Cargo.toml Cargo.lock ./
COPY libs/geo-core/ libs/geo-core/
COPY services/wms/ services/wms/
RUN cargo build --release -p wms

FROM alpine:3.21
RUN apk add --no-cache curl && adduser -D -u 10001 geoson
USER geoson
COPY --from=build /src/target/release/wms /usr/local/bin/wms
ENV GEOSON_HTTP_ADDR=:8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD curl -fsS http://localhost:8080/healthz || exit 1
ENTRYPOINT ["wms"]
