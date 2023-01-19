FROM golang:1.19 AS builder

RUN mkdir /app
WORKDIR /app


COPY go.mod go.sum ./

# install the dependencies without checking for go code
RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -installsuffix cgo -o /go/app ./cmd/multi-process-mode/main.go

# move the builded binary into the tiny alpine linux image
FROM alpine:latest
RUN apk --no-cache add ca-certificates && rm -rf /var/cache/apk/*
WORKDIR /app

COPY --from=builder /go/app ./
COPY --from=builder /usr/local/go/lib/time/zoneinfo.zip /usr/local/go/lib/time/zoneinfo.zip
CMD ["./app"]