FROM golang:1.21.1-alpine as dev-env

RUN apk add build-base

COPY . /go/src/
WORKDIR /go/src/

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN GOOS=linux GOARCH=amd64 go build -o ./main ./cmd/rotator/main.go

FROM alpine:latest



WORKDIR /go/src/

COPY --from=dev-env /go/src .

CMD ["./main"]