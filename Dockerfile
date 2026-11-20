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
ARG VERSION=dev
RUN VERSION_LDFLAG="-X github.com/QuietSugar/my-known-world/internal/version.Version=${VERSION}" \
    && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w ${VERSION_LDFLAG}" -o /out/myknownworld ./cmd/myknownworld \
    && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w ${VERSION_LDFLAG}" -o /out/myknownworld-mcp ./cmd/myknownworld-mcp \
    && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w ${VERSION_LDFLAG}" -o /out/myknownworld-markdown-import ./cmd/myknownworld-markdown-import \
    && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w ${VERSION_LDFLAG}" -o /out/myknownworld-cmdb ./cmd/myknownworld-cmdb \
    && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w ${VERSION_LDFLAG}" -o /out/myknownworld-materialize ./cmd/myknownworld-materialize

FROM alpine:3.22 AS runtime
ARG VERSION=dev
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
COPY --from=go-build --chown=10001:10001 /out/myknownworld-markdown-import /usr/local/bin/myknownworld-markdown-import
COPY --from=go-build --chown=10001:10001 /out/myknownworld-cmdb /usr/local/bin/myknownworld-cmdb
COPY --from=go-build --chown=10001:10001 /out/myknownworld-materialize /usr/local/bin/myknownworld-materialize
COPY --from=web-build --chown=10001:10001 /src/web/dist /app/web
ENV ADDR=:8080 \
    WEB_ROOT=/app/web \
    MYKNOWNWORLD_VERSION=${VERSION}
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
USER 10001:10001
ENTRYPOINT ["/sbin/tini","--","/usr/local/bin/myknownworld"]
