# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/orchestrator ./cmd/orchestrator

FROM alpine:3.20
RUN apk add --no-cache ca-certificates docker-cli
COPY --from=build /out/orchestrator /usr/local/bin/orchestrator
COPY configs/config.json /etc/homelab/config.json
ENV CONFIG_PATH=/etc/homelab/config.json
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/orchestrator"]
