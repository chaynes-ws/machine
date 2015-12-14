FROM golang:1.5.1

RUN go get  github.com/golang/lint/golint \
            github.com/mattn/goveralls \
            golang.org/x/tools/cover \
            github.com/tools/godep \
            github.com/aktau/github-release

ENV USER root
WORKDIR /go/src/github.com/chaynes-ws/machine

ADD . /go/src/github.com/chaynes-ws/machine
RUN mkdir bin
