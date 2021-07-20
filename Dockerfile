# Copyright 2020 NVIDIA
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
ARG BASE_IMAGE=alpine:3.14.0
FROM golang:alpine as builder

COPY . /usr/src/network-operator

ENV HTTP_PROXY $http_proxy
ENV HTTPS_PROXY $https_proxy

WORKDIR /usr/src/network-operator
RUN apk add --no-cache --virtual build-dependencies build-base linux-headers git && \
    make clean && \
    make build

FROM ${BASE_IMAGE}

ENV OPERATOR=/usr/local/bin/network-operator \
    USER_UID=1001 \
    USER_NAME=network-operator

# Copy manifest dir
COPY --from=builder /usr/src/network-operator/manifests /etc/manifests

# install operator binary
COPY --from=builder /usr/src/network-operator/build/_output/network-operator ${OPERATOR}
COPY --from=builder /usr/src/network-operator/build/bin /usr/local/bin
LABEL io.k8s.display-name="Mellanox Network Operator"
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]
USER ${USER_UID}
