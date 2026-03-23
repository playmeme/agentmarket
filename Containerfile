# Build the Go binary
FROM docker.io/golang:1.23-alpine AS builder

ARG VERSION=dev

WORKDIR /app

COPY backend/go.mod backend/go.sum* ./backend/
RUN cd backend && go mod download
COPY . .

# Build Go binary, inject Version
RUN cd backend && CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-X 'main.Version=${VERSION}'" \
    -o /app/main .

# Make a minimal production image
FROM alpine:latest
RUN apk --no-cache add curl
WORKDIR /app

# Copy only the Go binary. Static web files are mounted via Quadlet.
COPY --from=builder /app/main .

EXPOSE 8080
CMD ["./main"]
