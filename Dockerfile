# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.22
ARG ALPINE_VERSION=3.20
ARG UID=1000
ARG GID=1000

# ------------------------------------------------------------
# Stage 1 — Build
# ------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build

WORKDIR /src
RUN apk add --no-cache git

COPY src ./src

ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w" \
    -o /out/check_style ./src/check_style.go

# ------------------------------------------------------------
# Stage 2 — Runtime
# ------------------------------------------------------------
FROM alpine:${ALPINE_VERSION}

RUN apk add --no-cache bash grep sed coreutils findutils

ARG UID
ARG GID
RUN addgroup -g $GID app && adduser -D -u $UID -G app app

WORKDIR /app
ENV PATH="/app/bin:${PATH}"
RUN mkdir -p /app/bin /app/out && chown -R app:app /app

COPY --chmod=0755 checker.sh /app/checker.sh
COPY --from=build /out/check_style /app/bin/check_style

VOLUME ["/work", "/app/out"]

USER app

ENTRYPOINT ["/app/checker.sh"]
