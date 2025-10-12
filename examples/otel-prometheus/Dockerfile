# Stage 1: Build
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY main.go ./

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o app main.go

# Stage 2: Run
FROM alpine:3.19

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/app .

# Expose metrics port
EXPOSE 2112

# Run
CMD ["./app"]
