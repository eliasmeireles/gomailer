FROM golang:1.27.1-alpine3.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o app cmd/server/main.go

FROM alpine:3.24

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/app /bin/app

CMD ["/bin/app"]
