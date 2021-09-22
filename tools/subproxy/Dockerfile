FROM golang:alpine3.14 as BUILDER
WORKDIR /opt/test
COPY . .
WORKDIR /opt/test/tools/subproxy/
RUN go build

FROM alpine:3.13.5
COPY --from=BUILDER /opt/test/tools/subproxy/subproxy /bin/
ENTRYPOINT [ "/bin/subproxy" ]