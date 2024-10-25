# /Dockerfile

FROM golang:alpine

ADD ./ /go/src/app
WORKDIR /go/src/app

ENV PORT=8080
ENV GOPROXY=https://goproxy.cn,direct

CMD ["go", "run", "main.go"]