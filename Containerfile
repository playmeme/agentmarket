# Build the Go binary
FROM golang:1.22-alpine AS builder

ARG VERSION=dev

WORKDIR /app

# COPY go.mod go.sum ./
COPY go.mod ./
RUN go mod download
COPY . .

# Build Go binary, inject Version
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-X 'main.Version=${VERSION}'" \
    -o /app/main .


# Make a minimal production image
FROM alpine:latest

WORKDIR /root/
COPY --from=builder /app/main .
COPY --from=builder static ./static
EXPOSE 8080
CMD ["./main"]
