FROM golang:alpine AS builder
RUN apk add -U --no-cache ca-certificates

RUN mkdir -p /go/src/github.com/requilence/integram
WORKDIR /go/src/github.com/requilence/integram

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -installsuffix cgo -o /go/app github.com/requilence/integram/cmd/multi-process-mode

# move the builded binary into the scratch linux image
FROM scratch
WORKDIR /app

COPY --from=builder /go/app ./
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/local/go/lib/time/zoneinfo.zip /usr/local/go/lib/time/zoneinfo.zip
CMD ["./app"]