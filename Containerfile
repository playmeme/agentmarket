# ==========================================
# Stage 1: Build the Go binary
# ==========================================
FROM docker.io/golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod and sum files if you have them, and download dependencies
# COPY go.mod go.sum ./
# RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build the statically linked Go binary
# CGO_ENABLED=0 ensures it doesn't rely on host C libraries
RUN CGO_ENABLED=0 GOOS=linux go build -o static-server .

# ==========================================
# Stage 2: Create the minimal production image
# ==========================================
# We use 'scratch' (an empty image) for maximum security and minimal size
FROM scratch

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/static-server .

# Copy your static files over (adjust the source path if your folder is named differently)
COPY --from=builder /app/static ./static

# Expose the port your Go app listens on (assuming 8080)
EXPOSE 8080

# Run the binary
CMD ["./static-server"]