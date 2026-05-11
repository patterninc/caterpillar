FROM golang:1.24.7 AS builder

WORKDIR /go/src/github.com/patterninc/caterpillar

COPY . .

# build executable
RUN CGO_ENABLED=1 go build -o caterpillar ./cmd/caterpillar/caterpillar.go
RUN chmod 755 caterpillar

FROM debian:bookworm-slim
RUN mkdir -p /output
COPY --from=builder /go/src/github.com/patterninc/caterpillar/caterpillar .
