FROM daocloud.io/library/golang:1.6.0

WORKDIR /app/gopath/rddlock
ENV GOPATH /app/gopath

RUN go get github.com/everfore/rddlock/test

EXPOSE 8080

CMD ["/app/gopath/bin/test"]


