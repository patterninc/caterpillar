FROM golang:1.25-alpine AS builder

WORKDIR /go/src/github.com/patterninc/caterpillar

# Alpine 3.20 ships librdkafka 2.4.0; confluent-kafka-go v2 requires 2.14.0+.
RUN apk add --no-cache gcc musl-dev pkgconf \
    --repository=https://dl-cdn.alpinelinux.org/alpine/edge/community \
    librdkafka-dev

COPY . .

# build executable
RUN CGO_ENABLED=1 go build -tags dynamic -o caterpillar ./cmd/caterpillar/caterpillar.go
RUN chmod 755 caterpillar

FROM alpine:3.20 AS runtime
RUN apk add --no-cache \
    --repository=https://dl-cdn.alpinelinux.org/alpine/edge/community \
    librdkafka

RUN mkdir -p /output
COPY --from=builder /go/src/github.com/patterninc/caterpillar/caterpillar .

# caterpillar-operator sets only container args, so the EKS image names the binary itself.
FROM runtime AS eks
ENTRYPOINT ["/caterpillar"]

# Default image: no ENTRYPOINT, because ECS task definitions and downstream images supply their own command.
FROM runtime
