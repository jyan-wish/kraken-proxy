FROM --platform=$BUILDPLATFORM golang:1.14

ARG BUILDPLATFORM
ARG TARGETARCH
ARG TARGETOS

ENV GO111MODULE=on
WORKDIR /go/src/github.com/jyan-wish/kraken-proxy

# Cache dependencies
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . /go/src/github.com/jyan-wish/kraken-proxy

RUN CGO_ENABLED=0 GOARCH=${TARGETARCH} GOOS=${TARGETOS} go build -o kraken-proxy -a -installsuffix cgo .

FROM alpine:3.7
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=0 /go/src/github.com/jyan-wish/kraken-proxy/kraken-proxy /root/kraken-proxy
CMD /root/kraken-proxy
