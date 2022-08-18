FROM golang:1.18

WORKDIR /app

COPY . .

RUN go mod download

RUN go build -o server server/server.go

CMD ["server/server"]
