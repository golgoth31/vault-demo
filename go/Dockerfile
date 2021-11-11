# Get certificates for https conexions
FROM alpine:latest as certs
RUN apk add -U --no-cache ca-certificates

# build binary
FROM golang:1.16 as golang-build
ENV GOOS=linux
ENV GOARCH=amd64
ENV CGO_ENABLED=0
ENV GO111MODULE=on
ENV GOPROXY=https://proxy.golang.org
WORKDIR /go/src/github.com/golgoth31/vault-demo/go
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o ./main main.go

FROM scratch
VOLUME /data /config
ENV PATH=/bin
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=golang-build /go/src/github.com/golgoth31/vault-demo/go/main /main
EXPOSE 8080
ENTRYPOINT ["/main"]
