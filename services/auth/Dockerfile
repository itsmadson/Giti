# Build context must be the repo root: docker build -f services/auth/Dockerfile .
FROM golang:1.26-alpine AS build
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY GOWORK=off
WORKDIR /src
COPY libs/ogc-kit/ libs/ogc-kit/
COPY services/auth/ services/auth/
RUN go build -C services/auth -ldflags="-s -w" -o /out/auth .

FROM alpine:3.21
RUN apk add --no-cache curl ca-certificates && adduser -D -u 10001 giti
USER giti
COPY --from=build /out/auth /usr/local/bin/auth
ENV GITI_HTTP_ADDR=:8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD curl -fsS http://localhost:8080/healthz || exit 1
ENTRYPOINT ["auth"]
