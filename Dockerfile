# build stage
FROM golang:alpine AS build-env

ADD . /go/src/github.com/wenwu449/vsts-branch

WORKDIR /go/src/github.com/wenwu449/vsts-branch/
RUN go test -v && CGO_ENABLED=0 GGOS=linux go build -o vsts-branch

# final stage
FROM alpine

RUN apk add --no-cache ca-certificates
WORKDIR /vsts-branch
COPY --from=build-env /go/src/github.com/wenwu449/vsts-branch/vsts-branch .
COPY ./secrets.json .

CMD ["./vsts-branch"]