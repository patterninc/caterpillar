FROM golang:1.24.7-alpine AS builder

WORKDIR /go/src/github.com/patterninc/caterpillar

COPY . .

# build executable
RUN go build -o caterpillar ./cmd/caterpillar/caterpillar.go
RUN chmod 755 caterpillar

FROM alpine:3.20
RUN mkdir -p /output
COPY --from=builder /go/src/github.com/patterninc/caterpillar/caterpillar /patterninc/caterpillar/
