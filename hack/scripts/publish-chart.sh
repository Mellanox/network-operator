#!/bin/bash

#  2026 NVIDIA CORPORATION & AFFILIATES
#
#  Licensed under the Apache License, Version 2.0 (the License);
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an AS IS BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.

# Packages the Helm chart and pushes it to NGC, skipping the push if that chart version is
# already published.
#
# A chart version is regularly pushed more than once: a release pushes the commit to the release
# branch and then the tag on that same commit, and both events run the CI workflow, which
# publishes the same <chart-version>-<sha> chart. Re-running a workflow does the same.
#
# The push cannot be left to fail in that case, because NGC does not report a duplicate push
# consistently. It has answered one with "Chart upload failed: . Are you sure this is a valid
# packaged Helm chart?" minutes after the chart was accepted, and with the expected
# "already exists in the repository" for the very same push later on. Asking whether the chart
# version exists is unambiguous, so the push is skipped up front rather than pushed and the
# resulting error interpreted.
#
# Usage: NGC_REPO=<org/team/chart> VERSION=<chart version> [APP_VERSION=<app version>] publish-chart.sh

set -o nounset
set -o pipefail
set -o errexit

if [[ "${TRACE-0}" == "1" ]]; then
    set -o xtrace
fi

ATTEMPTS="${ATTEMPTS:-3}"
RETRY_DELAY_SECONDS="${RETRY_DELAY_SECONDS:-15}"

for required in NGC_REPO VERSION; do
    if [[ -z "${!required:-}" ]]; then
        echo "$required must be set" >&2
        exit 1
    fi
done

# A non-positive or non-numeric ATTEMPTS leaves the push loop with nothing to iterate over, which
# would report success without ever publishing the chart.
if [[ ! "$ATTEMPTS" =~ ^[1-9][0-9]*$ ]]; then
    echo "ATTEMPTS must be a positive integer, got '$ATTEMPTS'" >&2
    exit 1
fi

# chart_published reports whether the chart version is in the registry and fully uploaded.
#
# It must not report a false positive, since that skips a push that was needed and silently
# leaves the release without its chart. So a published chart has to both be reported by the
# registry and carry the UPLOAD_COMPLETE status: an unfinished upload, an error payload, or any
# failure to ask at all leaves the push to run, which is the operation that matters and which
# reports its own errors.
chart_published() {
    local info
    info="$(ngc registry chart info "$NGC_REPO:$VERSION" --format_type json 2>/dev/null)" || return 1
    grep -q 'UPLOAD_COMPLETE' <<<"$info"
}

if chart_published; then
    echo "chart version $VERSION is already published, nothing to do"
    exit 0
fi

error_log="$(mktemp)"
trap 'rm -f "$error_log"' EXIT

make chart-build

for attempt in $(seq 1 "$ATTEMPTS"); do
    # Capture stderr to a file rather than a process substitution, so that it is guaranteed to be
    # complete by the time it is inspected below.
    if make chart-push 2>"$error_log"; then
        cat "$error_log" >&2
        echo "published chart version $VERSION"
        exit 0
    fi
    cat "$error_log" >&2

    # A failed push does not mean the chart is missing: it may have been stored and then reported
    # as a failure, another run may have published it in the meantime, or it may have been
    # rejected as a duplicate. Ask the registry instead of reading the error, so that a version
    # which exists but never finished uploading is retried rather than taken for published.
    if chart_published; then
        echo "chart version $VERSION is published despite the reported failure, nothing to do"
        exit 0
    fi

    if [[ "$attempt" -eq "$ATTEMPTS" ]]; then
        echo "failed to push chart version $VERSION after $ATTEMPTS attempts" >&2
        exit 1
    fi

    echo "push attempt $attempt of $ATTEMPTS failed, retrying in ${RETRY_DELAY_SECONDS}s" >&2
    sleep "$RETRY_DELAY_SECONDS"
done
