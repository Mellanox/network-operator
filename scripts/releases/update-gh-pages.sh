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
Usage: $this [-h] [-p remote name] <helm-chart>

Options:
  -h         show this help and exit
  -p REMOTE  do git push to remote repo
EOF
}

#
# Argument parsing
#
while getopts "hap:" opt; do
    case $opt in
        h)  usage
            exit 0
            ;;
        p)  push_remote="$OPTARG"
            ;;
        *)  usage
            ;;
    esac
done

# Check that no extra args were provided
if [ $# -ne 1 ]; then
    echo "ERROR: extra positional arguments: $@"
    usage
#    exit 1
fi
echo $push_remote

chart="$3"
release=${chart::-4}

build_dir="/tmp/network-operator-build"

src_dir=$(pwd)

git worktree add $build_dir origin/gh-pages

# Drop worktree on exit
#trap "echo 'Removing Git worktree $build_dir'; git worktree remove '$build_dir'" EXIT


echo "-====="
echo $chart
echo $build_dir
echo $push_remote
echo "++++++++"

# Update Helm package index
mv $chart $build_dir/release
cd $build_dir/release
helm repo index . --url https://mellanox.github.io/network-operator/release --merge ./index.yaml

# Commit change
commit_msg="Release $release"
git add .
git commit -S -m "$commit_msg"

if [ -n "$push_remote" ]; then
    echo "Pushing gh-pages to $push_remote"
    git push "$push_remote" HEAD:gh-pages
fi
