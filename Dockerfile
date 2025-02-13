FROM golang:1.23.6-alpine3.21

WORKDIR /app

COPY . .

RUN go get -d -v ./...

RUN go build -o api .

EXPOSE 8000

CMD ["./api"]