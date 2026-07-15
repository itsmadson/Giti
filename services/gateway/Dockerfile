# Build context must be the repo root: docker build -f services/gateway/Dockerfile .
FROM golang:1.26-alpine AS build
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY
WORKDIR /src
COPY go.work go.work.sum* ./
COPY libs/ogc-kit/ libs/ogc-kit/
COPY services/gateway/ services/gateway/
RUN go build -C services/gateway -ldflags="-s -w" -o /out/gateway .

FROM alpine:3.21
RUN apk add --no-cache curl ca-certificates && adduser -D -u 10001 geoson
USER geoson
COPY --from=build /out/gateway /usr/local/bin/gateway
ENV GEOSON_HTTP_ADDR=:8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD curl -fsS http://localhost:8080/healthz || exit 1
ENTRYPOINT ["gateway"]
