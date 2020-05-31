FROM golang:alpine as builder

ADD . /usr/src/mellanox-network-operator

ENV HTTP_PROXY $http_proxy
ENV HTTPS_PROXY $https_proxy

RUN apk add --update --virtual build-dependencies build-base linux-headers && \
    cd /usr/src/mellanox-network-operator && \
    make clean && \
    make build

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ENV OPERATOR=/usr/local/bin/network-operator \
    USER_UID=1001 \
    USER_NAME=network-operator

# install operator binary
COPY --from=builder /usr/src/mellanox-network-operator/build/_output/network-operator ${OPERATOR}

COPY --from=builder /usr/src/mellanox-network-operator/build/bin /usr/local/bin
LABEL io.k8s.display-name="Mellanox Network Operator"

RUN  /usr/local/bin/user_setup
ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
