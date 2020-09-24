FROM golang:1.15-buster as builder

WORKDIR /app
COPY go.* ./
RUN go mod download

COPY . ./

ARG rev=development
ARG region=unknown
RUN go build -mod=readonly -v -o server -ldflags="-X 'main.Version=v1.0.0-${rev}' -X 'main.Region=${region}'"

FROM debian:buster-slim
RUN set -x && apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y \
    ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/server /app/server

# Run the web service on container startup.
CMD ["/app/server"]