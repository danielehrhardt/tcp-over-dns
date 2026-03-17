# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath -o tcpdns ./cmd/tcpdns

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache \
    iodine \
    openssh-client \
    iptables \
    bash \
    ca-certificates

COPY --from=builder /app/tcpdns /usr/local/bin/tcpdns
COPY scripts/vps-setup.sh /usr/local/bin/vps-setup.sh
RUN chmod +x /usr/local/bin/vps-setup.sh

# TUN device needed for iodine
RUN mkdir -p /dev/net && \
    mknod /dev/net/tun c 10 200 2>/dev/null || true

EXPOSE 53/udp

ENTRYPOINT ["tcpdns"]
CMD ["--help"]
