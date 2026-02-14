# Use a multi-stage build to keep the final image small
# Start from the latest Golang base image
# We use --platform=$BUILDPLATFORM to run the build process on the builder's native architecture (faster)
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set the working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
# CGO_ENABLED=0 ensures a statically linked binary
# GOOS and GOARCH ensure we target the correct platform
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /app/pb_app main.go

# Start a new stage from scratch
FROM alpine:latest

# Install necessary runtime dependencies
RUN apk add --no-cache ca-certificates

# Set working directory
WORKDIR /pb

# Copy the compiled binary from the builder stage
COPY --from=builder /app/pb_app /pb/pocketbase

# Expose port 8090
EXPOSE 8090

# Command to run the executable
CMD ["/pb/pocketbase", "serve", "--http=0.0.0.0:8090"]
