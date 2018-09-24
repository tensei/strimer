FROM golang:1.11

WORKDIR /go/src/strimer
COPY ./src .

RUN go get -v ./...
RUN go install -v ./...

CMD ["strimer"]