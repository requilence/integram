FROM golang:1.9 AS builder

RUN curl -fsSL -o /usr/local/bin/dep https://github.com/golang/dep/releases/download/v0.3.2/dep-linux-amd64 && chmod +x /usr/local/bin/dep

RUN mkdir -p /go/src/github.com/requilence/integram
WORKDIR /go/src/github.com/requilence/integram

COPY Gopkg.toml Gopkg.lock ./

# install the dependencies without checking for go code
RUN dep ensure -vendor-only

COPY . ./

RUN go build -o /go/app github.com/requilence/integram
CMD ["/go/app"]