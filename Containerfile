# Build the Go binary
FROM docker.io/golang:1.25-alpine AS builder

ARG VERSION=dev

WORKDIR /srv

COPY backend/go.mod backend/go.sum* ./backend/
RUN cd backend && go mod download
COPY . .

# Build Go binary, inject Version
RUN cd backend && CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-X 'main.Version=${VERSION}'" \
    -o /srv/main .

# Make a minimal production image
FROM alpine:latest
RUN apk --no-cache add curl
WORKDIR /srv

# Copy only the Go binary. The frontend files are mounted via Quadlet.
COPY --from=builder /srv/main .

EXPOSE 8080
CMD ["./main"]
