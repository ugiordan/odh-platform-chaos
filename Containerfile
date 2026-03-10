# Stage 1: Build
FROM golang:1.25 AS builder

WORKDIR /workspace

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

ARG VERSION=dev

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X github.com/opendatahub-io/odh-platform-chaos/internal/cli.Version=${VERSION}" \
    -o /odh-chaos ./cmd/odh-chaos

# Stage 2: Runtime
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /odh-chaos /odh-chaos
COPY --from=builder /workspace/knowledge/ /knowledge/
COPY --from=builder /workspace/experiments/ /experiments/

USER 65532:65532

ENTRYPOINT ["/odh-chaos"]
