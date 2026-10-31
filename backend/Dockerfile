# syntax=docker/dockerfile:1

# Dependency layer: changes only when go.mod/go.sum change.
# Keep this in sync with the `go` directive in go.mod (currently 1.25). The
# official golang images set GOTOOLCHAIN=local, so a mismatch is a hard error.
FROM golang:1.25-alpine AS deps

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Test layer: used by `make test` and CI. Never shipped.
FROM deps AS test

COPY . .
CMD ["sh", "-c", "go vet ./... && go test ./..."]

# Build layer: produces the two binaries.
FROM deps AS build

COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
ENV CGO_ENABLED=0

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath \
        -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
        -o /out/api ./cmd/api && \
    go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

# Runtime layer: minimal, non-root, with CA certificates and timezone data.
FROM alpine:3.20 AS runtime

RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=build /out/api /app/api
COPY --from=build /out/seed /app/seed

USER app
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 \
    CMD wget -qO- http://localhost:8080/healthz >/dev/null || exit 1

ENTRYPOINT ["/app/api"]
