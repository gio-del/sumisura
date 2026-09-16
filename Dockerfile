# The release image (ADR-0039): the Go backend plus the production frontend
# build it serves, on one port. Build context is the repo root.
#
# Development is unchanged and does not use this file — docker-compose.yml
# still builds backend/Dockerfile and runs the Vite dev server beside it.

FROM node:22-slim AS frontend
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/. .
RUN npm run build

FROM golang:1.27 AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/. .
# Stamped by the release workflow (--build-arg VERSION=x.y.z); an unset
# VERSION leaves the binary reporting "dev", as a local build should.
ARG VERSION=dev
# TARGETARCH is supplied by buildx for each platform it builds; CGO stays
# off so the result is a static binary that needs no cross-compiler.
ARG TARGETARCH
RUN CGO_ENABLED=0 GOARCH=${TARGETARCH} go build \
    -ldflags "-X github.com/gio-del/sumisura/backend/internal/version.Version=${VERSION}" \
    -o /out/server ./cmd/server

# Same base and the same pinned tooling as backend/Dockerfile: Render shells
# out to typst and the ATS-parsability check to pdftotext, and render
# fidelity must not differ between a dev container and a release image
# (ADR-0012, ADR-0016). The typst version comes from typst-version.txt at
# the repo root — bump it there, never here.
FROM debian:bookworm-slim
ARG TARGETARCH
COPY typst-version.txt /tmp/typst-version.txt
RUN . /tmp/typst-version.txt \
    && case "${TARGETARCH}" in \
         amd64) TYPST_ARCH=x86_64 ;; \
         arm64) TYPST_ARCH=aarch64 ;; \
         *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
       esac \
    && apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl xz-utils fonts-liberation poppler-utils=22.12.0-2+deb12u3 \
    && curl -fsSL "https://github.com/typst/typst/releases/download/v${TYPST_VERSION}/typst-${TYPST_ARCH}-unknown-linux-musl.tar.xz" \
      -o /tmp/typst.tar.xz \
    && tar -xf /tmp/typst.tar.xz -C /tmp \
    && mv "/tmp/typst-${TYPST_ARCH}-unknown-linux-musl/typst" /usr/local/bin/typst \
    && apt-get purge -y curl xz-utils \
    && apt-get autoremove -y \
    && rm -rf /var/lib/apt/lists/* /tmp/typst* /tmp/typst-version.txt

WORKDIR /workspace
# template/ is part of the image: it is code (pure presentation, ADR-0012),
# not user data. data/ and output/ are volumes the self-hoster supplies.
COPY template/ /workspace/template/
COPY --from=backend /out/server /usr/local/bin/sumisura
COPY --from=frontend /app/dist /workspace/web

ENV DATA_DIR=/workspace/data \
    PROJECT_ROOT=/workspace \
    STATIC_DIR=/workspace/web \
    PORT=8080
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/sumisura"]
