# Build stage
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder
ARG TARGETARCH

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
# We build the binary named "server" from the main package in cmd/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o server ./cmd/server

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the Pre-built binary from the previous stage
COPY --from=builder /app/server .

# Expose port 8008 to the outside world
EXPOSE 8008

# Command to run the executable
CMD ["./server"]
