# Go Integration Coverage on OpenShift

This directory documents how to collect runtime coverage from the
coverage-enabled OpenShift bundle (`network-operator-bundle:<version>-coverage`).

## Build the coverage bundle locally

```bash
# Use the coverage operator image tag or digest published by CI.
export TAG=nvcr.io/nvstaging/mellanox/network-operator:<sha>-coverage
export NETWORK_OPERATOR_VERSION=<sha>-coverage
export VERSION=26.7.0-beta.5   # same as production bundle version

make bundle-coverage
make bundle-coverage-build BUNDLE_COVERAGE_IMG=network-operator-bundle:local-coverage
```

The coverage bundle is generated into `bundle-coverage/` and is not committed.
The production `bundle/` directory is unchanged.

Install via a dedicated CatalogSource or `operator-sdk run bundle` against the
coverage bundle image. Do not install production and coverage bundles with the
same package version in one catalog.

## What the coverage bundle adds

Compared to the production bundle, the CSV deployment includes:

- `GOCOVERDIR=/coverage`
- `COVERAGE_CLEAR_AFTER_FLUSH=1`
- an `emptyDir` volume mounted at `/coverage`
- the `-coverage` operator image in the deployment and `relatedImages`

These mirror the Helm `operator.coverage.enabled` overlay.

## Flush coverage on OpenShift

The operator image is distroless, so `oc exec` cannot send signals or run
shell commands inside the manager container. Flush coverage from the node
instead:

1. Find the operator pod and node:
   ```bash
   NS=nvidia-network-operator
   POD=$(oc -n $NS get pod -l control-plane=nvidia-network-operator-controller \
     -o jsonpath='{.items[0].metadata.name}')
   NODE=$(oc -n $NS get pod $POD -o jsonpath='{.spec.nodeName}')
   POD_UID=$(oc -n $NS get pod $POD -o jsonpath='{.metadata.uid}')
   CID=$(oc -n $NS get pod $POD -o jsonpath='{.status.containerStatuses[0].containerID}' \
     | sed 's|.*://||')
   ```

2. Send SIGUSR1 to the manager process:
   ```bash
   oc debug node/$NODE -- chroot /host bash -c \
     "PID=\$(crictl inspect --output go-template --template '{{.info.pid}}' $CID); kill -USR1 \$PID; sleep 3"
   ```

3. Read `covcounters.*` from the kubelet emptyDir path:
   ```bash
   VOL=/var/lib/kubelet/pods/$POD_UID/volumes/kubernetes.io~empty-dir/go-coverage
   oc debug node/$NODE -- chroot /host bash -c "ls -la $VOL"
   oc debug node/$NODE -- chroot /host bash -c "tar -C $VOL -cf - ." > covdata.tar
   mkdir covdata && tar -xf covdata.tar -C covdata
   ```

4. Convert to reports:
   ```bash
   go tool covdata percent -i=covdata
   go tool covdata textfmt -i=covdata -o=covdata.out
   go tool cover -func=covdata.out
   ```

Or use the helper script. It clears prior `covmeta.*` and `covcounters.*` files in the output directory before extracting, so a reused directory cannot merge stale executions:

```bash
chmod +x scripts/e2e-coverage/flush-openshift.sh
./scripts/e2e-coverage/flush-openshift.sh nvidia-network-operator
```

## Verify the operator started with coverage enabled

```bash
oc -n nvidia-network-operator logs deploy/nvidia-network-operator-controller-manager \
  | grep 'coverage flush handler enabled'
oc -n nvidia-network-operator get pod -l control-plane=nvidia-network-operator-controller \
  -o jsonpath='{range .items[0].spec.containers[0].env[*]}{.name}={.value}{"\n"}{end}' \
  | grep GOCOVERDIR
```
