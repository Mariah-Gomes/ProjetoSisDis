# ---- build ----
FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache zeromq-dev pkgconf build-base

# 1) módulos básicos
COPY go.mod ./
RUN go mod download

# 2) código
COPY . .

# 3) força a resolução das deps usadas no código (gera go.sum)
RUN go get github.com/pebbe/zmq4@v1.4.0 github.com/vmihailenco/msgpack/v5@v5.4.0
RUN go mod tidy

# 4) build
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /bin/auto_client

# ---- runtime ----
FROM alpine:3.20
RUN apk add --no-cache zeromq
COPY --from=build /bin/auto_client /usr/local/bin/auto_client
ENTRYPOINT ["auto_client"]
