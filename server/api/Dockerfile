FROM golang:1.25-alpine AS deps

WORKDIR /src

COPY go.work go.work
COPY go.work.sum go.work.sum
COPY server/api/go.mod server/api/go.mod
COPY server/worker/go.mod server/worker/go.mod
COPY server/shared/go.mod server/shared/go.mod

# Cache /go/pkg/mod so module downloads persist across builds
RUN --mount=type=cache,target=/go/pkg/mod \
    go work edit -dropuse=./scripts && \
    go work sync && \
    cd server/api && go mod download && \
    cd ../worker && go mod download && \
    cd ../shared && go mod download

# ─── Stage 2: build ──────────────────────────────────────────────────────────
FROM deps AS builder

COPY server/shared/ server/shared/
COPY server/api/ server/api/

# Cache both module cache and the Go build cache (compiled pkgs)
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd server/api && \
    CGO_ENABLED=0 GOOS=linux GOMEMLIMIT=512MiB GOGC=50 \
    go build \
      -p 2 \
      -trimpath \
      -ldflags="-s -w" \
      -o /bin/api \
      ./cmd/api/main.go

# ─── Stage 3: minimal runtime ────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS runner

COPY --from=builder /bin/api /bin/api

EXPOSE 8080

ENTRYPOINT ["/bin/api"]
