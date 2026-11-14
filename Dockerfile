#
# One image definition for every mall Go binary. The binary to build is chosen
# at build time through the SERVICE build argument:
#
#   docker build --build-arg SERVICE=identity -t mall/identity .
#   docker build --build-arg SERVICE=gateway  -t mall/gateway  .
#
# deploy/compose/services.yml passes SERVICE for each service. Building with a
# missing or unknown SERVICE fails loudly instead of producing an empty image.

FROM golang:1.27-alpine AS builder

ARG SERVICE
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}
RUN test -n "$SERVICE" || ( \
        echo "SERVICE build arg is required, e.g. --build-arg SERVICE=identity" >&2; \
        echo "known services: gateway admin identity catalog inventory cart pricing order payment fulfillment aftersales" >&2; \
        exit 1; \
    )

WORKDIR /src

# Keep module downloads in their own layer: source edits do not invalidate it.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# No cgo is required: the MySQL driver, JWT, bcrypt and the DTM client are all
# pure Go. The resulting binary runs on the minimal Alpine runtime below.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app "./cmd/${SERVICE}"

FROM alpine:3.20 AS runtime

# ca-certificates/tzdata for outbound TLS and DATETIME loc=Local handling;
# wget backs the compose healthchecks. Run as an unprivileged user.
RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app

WORKDIR /app
COPY --from=builder /out/app /app/app

USER 10001:10001

# gRPC services use 900x, health/metrics 910x, gateway 8080, admin 8081.
EXPOSE 8080 8081 9000 9100

ENTRYPOINT ["/app/app"]
