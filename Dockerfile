FROM golang:1.11

WORKDIR /go/src/strimer
COPY ./src .

RUN go get -v ./...
RUN go install -v ./...
RUN apt-get update && apt-get install -y ffmpeg

CMD ["strimer"]