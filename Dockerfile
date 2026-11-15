FROM golang:1.27-alpine AS builder

WORKDIR /app
COPY go.mod ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o path-api .

FROM alpine:3

WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/path-api .
