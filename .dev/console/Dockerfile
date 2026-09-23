FROM golang:1.27.1-alpine3.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go test ./... && CGO_ENABLED=0 go build -o console .

FROM alpine:3.24

COPY --from=builder /app/console /bin/console

EXPOSE 8080
CMD ["/bin/console"]
