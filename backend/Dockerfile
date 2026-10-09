# Build stage
FROM golang:1.26-alpine AS build
WORKDIR /src

# Download dependencies first so this layer is cached until go.mod changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/web ./cmd/web

# Runtime stage
FROM alpine:3.22
RUN adduser -D -H -u 10001 app
USER app
COPY --from=build /out/web /usr/local/bin/web

ENV GIN_MODE=release
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/web"]
