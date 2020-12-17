FROM golang:alpine as builder

COPY . /usr/src/dummy-cni

WORKDIR /usr/src/dummy-cni
RUN apk add --no-cache --virtual build-dependencies build-base=~0.5 && \
    make clean && \
    make build

FROM alpine:3
COPY --from=builder /usr/src/dummy-cni/bin/dummy-cni /usr/bin/
WORKDIR /

LABEL io.k8s.display-name="DUMMY CNI"

COPY ./images/entrypoint.sh /

ENTRYPOINT ["/entrypoint.sh"]
