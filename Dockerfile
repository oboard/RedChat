# /Dockerfile

FROM golang:alpine

ADD ./ /go/src/app
WORKDIR /go/src/app

ENV PORT=8080
ENV GOPROXY=https://goproxy.cn,direct
RUN go build -o /go/bin/app
# 暴露应用程序的端口
EXPOSE 8080
# 设置启动命令
CMD ["/go/bin/app"]