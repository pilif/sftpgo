FROM golang:stretch
RUN mkdir /app
ADD . /app/
WORKDIR /app
RUN go build .
