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

# Ensure the final image uses the correct platform
ARG ARCH

# Build the manager binary
FROM golang:1.23@sha256:d56c3e08fe5b27729ee3834854ae8f7015af48fd651cd25d1e3bcf3c19830174 AS manager-builder

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download -x

# Copy the go source
COPY ./ ./

# Build
ARG ARCH
ARG LDFLAGS
ARG GCFLAGS
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -ldflags="${LDFLAGS}" -gcflags="${GCFLAGS}" -o manager main.go  && \
    CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -ldflags="${LDFLAGS}" -gcflags="${GCFLAGS}" -o keep-ncp cmd/keep-ncp/main.go


# Build the apply-crds binary
FROM golang:1.23@sha256:d56c3e08fe5b27729ee3834854ae8f7015af48fd651cd25d1e3bcf3c19830174 AS apply-crds-builder

WORKDIR /workspace

# Copy the Go Modules manifests
COPY cmd/apply-crds/go.mod go.mod
COPY cmd/apply-crds/go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download -x

# Copy the go source
COPY cmd/apply-crds/ ./
COPY deployment/network-operator/ ./network-operator-chart/

# copy CRDs from helm charts
RUN mkdir crds && \
    cp -r network-operator-chart/crds /workspace/crds/network-operator/ && \
    cp -r network-operator-chart/charts/sriov-network-operator/crds /workspace/crds/sriov-network-operator/ && \
    cp -r network-operator-chart/charts/node-feature-discovery/crds /workspace/crds/node-feature-discovery/ && \
    cp -r network-operator-chart/charts/nic-configuration-operator-chart/crds /workspace/crds/nic-configuration-operator/

# Build
ARG ARCH
ARG LDFLAGS
ARG GCFLAGS
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -ldflags="${LDFLAGS}" -gcflags="${GCFLAGS}" -o apply-crds main.go

FROM --platform=linux/${ARCH} registry.access.redhat.com/ubi8-micro:8.10

WORKDIR /
COPY --from=manager-builder /workspace/manager .
COPY --from=manager-builder /workspace/keep-ncp .
COPY --from=apply-crds-builder /workspace/apply-crds .
COPY --from=apply-crds-builder /workspace/crds /crds

# Default Certificates are missing in micro-ubi. These are need to fetch DOCA drivers image tags
COPY --from=manager-builder /etc/ssl/certs/ca-certificates.crt /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem
COPY /webhook-schemas /webhook-schemas
COPY manifests/ manifests/
USER 65532:65532

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

ENTRYPOINT ["/manager"]
