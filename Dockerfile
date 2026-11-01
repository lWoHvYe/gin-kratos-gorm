FROM golang:1.26-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        netbase \
        curl \
        && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /src/bin /app/bin
COPY --from=builder /src/configs /app/configs

EXPOSE 8000 9000
VOLUME ["/data/conf"]

# The monolith is the default entrypoint. Override the command to run
# /app/bin/order or /app/bin/product for split deployments.
CMD ["/app/bin/server", "-conf", "/data/conf/config.yaml"]
