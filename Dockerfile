FROM golang:1.26-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /zynqel-core ./cmd/zynqel-core

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /zynqel-core /usr/local/bin/zynqel-core
COPY web/ /app/web/

WORKDIR /app
EXPOSE 8080

CMD ["zynqel-core"]
