FROM daocloud.io/library/golang:1.6.0

WORKDIR /app/gopath/rddlock
ENV GOPATH /app/gopath

RUN git clone --depth 1 git://github.com/everfore/rddlock.git . && go get github.com/beego/mux && go build -o rddlock ./test

EXPOSE 8080

CMD ["/app/gopath/rddlock/test/rddlock"]


