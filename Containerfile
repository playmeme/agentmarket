# Build the Go binary
FROM docker.io/golang:1.23-alpine AS builder

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
FROM scratch
WORKDIR /app
COPY --from=builder /app/main .
COPY --from=builder /app/static ./static
EXPOSE 8080
CMD ["./main"]
