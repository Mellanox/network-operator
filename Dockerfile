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

# Build the manager binary
FROM golang:1.20 as builder

WORKDIR /workspace
# Add kubectl tool
ARG TARGETPLATFORM
ENV TARGETPLATFORM=${TARGETPLATFORM:-linux/amd64}
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/${TARGETPLATFORM}/kubectl"
RUN chmod +x ./kubectl

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download -x


# Copy the go source
COPY ./ ./

# copy CRDs from helm charts
RUN mkdir crds && \
    cp -r deployment/network-operator/crds /workspace/crds/network-operator/ && \
    cp -r deployment/network-operator/charts/sriov-network-operator/crds /workspace/crds/sriov-network-operator/ && \
    cp -r deployment/network-operator/charts/node-feature-discovery/crds /workspace/crds/node-feature-discovery/

# Build
ARG ARCH ?= $(shell go env GOARCH)
ARG LDFLAGS
ARG GO_BUILD_GC_ARGS
RUN --mount=type=cache,target=/root/.cache/go-build \
        --mount=type=cache,target=/go/pkg/mod \
      CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -ldflags="${LDFLAGS}" -gcflags="${GO_BUILD_GC_ARGS}" -o manager main.go


FROM registry.access.redhat.com/ubi8-micro:8.8

ARG BUILD_DATE
ARG VERSION
ARG VCS_REF
ARG VCS_BRANCH

LABEL version=$VERSION
LABEL vcs-type="git"
LABEL vcs-branch=$VCS_BRANCH
LABEL vcs-ref=$VCS_REF
LABEL build-date=$BUILD_DATE
LABEL io.k8s.display-name="NVIDIA Network Operator"
LABEL name="NVIDIA Network Operator"
LABEL vendor="NVIDIA"
LABEL release="N/A"
LABEL summary="Deploy and manage NVIDIA networking resources in Kubernetes"
LABEL description="NVIDIA Network Operator"
LABEL io.k8s.description="NVIDIA Network Operator"
LABEL maintainer="NVIDIA nvidia-network-operator-support@nvidia.com"
LABEL url="https://github.com/Mellanox/network-operator"
LABEL org.label-schema.vcs-url="https://github.com/Mellanox/network-operator"

WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/kubectl /usr/local/bin
COPY --from=builder /workspace/crds /crds

COPY /webhook-schemas /webhook-schemas
COPY manifests/ manifests/
USER 65532:65532

ENTRYPOINT ["/manager"]
