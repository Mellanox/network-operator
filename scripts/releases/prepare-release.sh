#!/bin/bash -e
# Copyright 2021 NVIDIA
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

set -o pipefail

this=`basename $0`

usage () {
cat << EOF
Usage: $this [-h] [-p remote name] RELEASE_VERSION GPG_KEY

Options:
  -h         show this help and exit
  -p REMOTE  do git push to remote repo

Example:

  $this v0.1.2 "Jane Doe <jane.doe@example.com>"


NOTE: The GPG key should be associated with the signer's Github account.
EOF
}

sign_helm_chart() {
  local chart="$1"
  echo "Signing Helm chart $chart"
  local sha256=`openssl dgst -sha256 "$chart" | awk '{ print $2 }'`
  local yaml=`tar xf $chart -O network-operator/Chart.yaml`
  echo "$yaml
...
files:
  $chart: sha256:$sha256" | gpg -u "$key" --clearsign -o "$chart.prov"
}

#
# Parse command line
#
while getopts "h" opt; do
    case $opt in
        h)  usage
            exit 0
            ;;
        p)  push_remote="$OPTARG"
            ;;
        *)  usage
            exit 1
            ;;
    esac
done
shift "$((OPTIND - 1))"

# Check that no extra args were provided
if [ $# -ne 2 ]; then
    if [ $# -lt 2 ]; then
        echo -e "ERROR: too few arguments\n"
    else
        echo -e "ERROR: unknown arguments: ${@:3}\n"
    fi
    usage
    exit 1
fi

release=$1
key="$2"
shift 2

container_image=k8s.gcr.io/nfd/node-feature-discovery:$release

#
# Check/parse release number
#
if [ -z "$release" ]; then
    echo -e "ERROR: missing RELEASE_VERSION\n"
    usage
    exit 1
fi

if [[ $release =~ ^(v[0-9]+\.[0-9]+)(\..+)?$ ]]; then
    docs_version=${BASH_REMATCH[1]}
    semver=${release:1}
else
    echo -e "ERROR: invalid RELEASE_VERSION '$release'"
    exit 1
fi

# Patch Helm chart
chart=`echo $release | cut -c 2-`
sed -e s"/appVersion:.*/appVersion: $release/" \
    -e s"/^version:.*/version: $chart/" \
    -i deployment/network-operator/Chart.yaml
sed -e s"/pullPolicy:.*/pullPolicy: IfNotPresent/" \
    -i deployment/network-operator/values.yaml

# Commit changes
#git add .
#git commit -S -m "Release $release"

#if [ -n "$push_remote" ]; then
#    echo "Pushing to $push_remote"
#    git push "$push_remote"
#fi

#
# Create release assets to be uploaded
#
VERSION=$semver make chart-build

chart_name="network-operator-$semver.tgz"
sign_helm_chart $chart_name

cat << EOF

*******************************************************************************
*** Please manually upload the following generated files to the Github release
*** page:
***
***   $chart_name
***   $chart_name.prov
***
*******************************************************************************
EOF

