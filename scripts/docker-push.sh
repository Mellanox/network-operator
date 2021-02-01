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
#!/bin/bash
IMAGE_NAME=$1
VERSION=$2
CPU_ARCH=$3
export DOCKER_CLI_EXPERIMENTAL="enabled";
docker tag $IMAGE_NAME ${IMAGE_NAME}-${CPU_ARCH}:${VERSION}
docker push ${IMAGE_NAME}-${CPU_ARCH}:${VERSION}
docker manifest create ${IMAGE_NAME}:${VERSION} ${IMAGE_NAME}-${CPU_ARCH}:${VERSION}
docker manifest annotate ${IMAGE_NAME}:${VERSION} ${IMAGE_NAME}-${CPU_ARCH}:${VERSION} --arch ${CPU_ARCH}
docker manifest push ${IMAGE_NAME}:${VERSION}
