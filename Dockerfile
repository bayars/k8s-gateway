# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git protobuf-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o gateway ./cmd/gateway

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates openssh-client

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/gateway .

# Copy configuration
COPY config/ ./config/

# Create logs directory
RUN mkdir -p /root/logs

# Expose ports
# gRPC
EXPOSE 50051
# gNMI
EXPOSE 57400
# SSH
EXPOSE 2222
# NETCONF (if needed for direct access)
EXPOSE 830

# Run the gateway
CMD ["./gateway"]
