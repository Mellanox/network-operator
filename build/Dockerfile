FROM golang:alpine as builder

ADD . /usr/src/nic-operator

ENV HTTP_PROXY $http_proxy
ENV HTTPS_PROXY $https_proxy

RUN apk add --update --virtual build-dependencies build-base linux-headers && \
    cd /usr/src/nic-operator && \
    make clean && \
    make build

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ENV OPERATOR=/usr/local/bin/nic-operator \
    USER_UID=1001 \
    USER_NAME=nic-operator

# install operator binary
COPY --from=builder /usr/src/nic-operator/build/_output/nic-operator ${OPERATOR}

COPY --from=builder /usr/src/nic-operator/build/bin /usr/local/bin
LABEL io.k8s.display-name="NIC Operator"

RUN  /usr/local/bin/user_setup
ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
