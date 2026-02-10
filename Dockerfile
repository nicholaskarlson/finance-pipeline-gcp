# Build stage
FROM golang:1.22-bookworm AS builder
WORKDIR /src

COPY go.mod ./
RUN go mod download || true

COPY . .

# Build pipeline binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/pipeline ./cmd/pipeline

# Install pinned book snapshot tools into /out/bin
RUN mkdir -p /out/bin && \
    GOBIN=/out/bin CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
      go install github.com/nicholaskarlson/proof-first-recon/cmd/recon@book-v1 && \
    GOBIN=/out/bin CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
      go install github.com/nicholaskarlson/proof-first-auditpack/cmd/auditpack@book-v1

# Runtime stage (no package manager, includes CA certs)
FROM gcr.io/distroless/base-debian12

COPY --from=builder /out/pipeline /usr/local/bin/pipeline
COPY --from=builder /out/bin/recon /usr/local/bin/recon
COPY --from=builder /out/bin/auditpack /usr/local/bin/auditpack

ENTRYPOINT ["/usr/local/bin/pipeline"]
