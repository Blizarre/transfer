# syntax=docker/dockerfile:1

FROM golang:1.17-alpine AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY cmd/ ./

RUN go build -o /server ./server/

FROM alpine:latest
COPY --from=build /server /server
EXPOSE 1234
CMD [ "/server" ]