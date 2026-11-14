# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM node:22-alpine AS web-build
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS go-build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o /out/myknownworld ./cmd/myknownworld \
    && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o /out/myknownworld-mcp ./cmd/myknownworld-mcp

FROM alpine:3.22 AS runtime
ARG VERSION=0.0.2
ARG VCS_REF=unknown
LABEL org.opencontainers.image.title="MyKnownWorld" \
      org.opencontainers.image.description="Personal cognitive graph web application and API" \
      org.opencontainers.image.source="https://github.com/QuietSugar/my-known-world" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${VCS_REF}"
RUN apk add --no-cache ca-certificates tini \
    && addgroup -g 10001 -S myknownworld \
    && adduser -u 10001 -S -G myknownworld -h /app myknownworld
WORKDIR /app
COPY --from=go-build --chown=10001:10001 /out/myknownworld /usr/local/bin/myknownworld
COPY --from=go-build --chown=10001:10001 /out/myknownworld-mcp /usr/local/bin/myknownworld-mcp
COPY --from=web-build --chown=10001:10001 /src/web/dist /app/web
ENV ADDR=:8080 \
    WEB_ROOT=/app/web
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
USER 10001:10001
ENTRYPOINT ["/sbin/tini","--","/usr/local/bin/myknownworld"]
