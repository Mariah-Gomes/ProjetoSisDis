# ---- build ----
FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache zeromq-dev pkgconf build-base

COPY go.mod ./
RUN go mod download

COPY . .

# garante go.sum com as deps reais
RUN go get github.com/pebbe/zmq4@v1.4.0 github.com/vmihailenco/msgpack/v5@v5.4.0
RUN go mod tidy

RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /bin/client

# ---- runtime ----
FROM alpine:3.20
RUN apk add --no-cache zeromq
COPY --from=build /bin/client /usr/local/bin/client
ENTRYPOINT ["client"]
