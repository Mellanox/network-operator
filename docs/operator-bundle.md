# Operator build and deployment


## Building the operator bundle

For development and testing purposes it may be beneficial to build the operator bundle.

The template for the CSV is located [here](config/manifests/bases/nvidia-network-operator.clusterserviceversion.yaml).

Build the bundle:

**Note**:
- `VERSION` should be a valid semantic version
- `DEFAULT_CHANNEL` should be `stable`
- `CHANNELS` should include the `DEFAULT_CHANNEL` value and `stable` seperated by a comma
- `TAG` should use SHA256

Here how to obtain the digest:

```bash
skopeo inspect docker://nvcr.io/nvidia/cloud-native/network-operator:v1.1.0 | jq .Digest
"sha256:17afa53f1cf3733c8d0cd282c565975ed5de3124dfc2b7c485ad12c97e51c251"
```

```bash
DEFAULT_CHANNEL=stable CHANNELS=v1.1,stable VERSION=1.1.0 TAG=nvcr.io/nvidia/cloud-native/network-operator@sha256:17afa53f1cf3733c8d0cd282c565975ed5de3124dfc2b7c485ad12c97e51c251 make bundle
```

Build the bundle image:

```bash
BUNDLE_IMG=mellanox/network-operator-bundle-1.1.0 make bundle-build
```

Push the bundle image:

```bash
BUNDLE_IMG=mellanox/network-operator-bundle-1.1.0 make bundle-push
```

## Deploying the operator

The operator must be deployed to the nvidia-network-operator namespace. Create the namespace.

```bash
cat <<EOF | kubectl create -f -
apiVersion: v1
kind: Namespace
metadata:
  name: nvidia-network-operator
  labels:
    name: nvidia-network-operator
EOF
```

### Download operator-sdk

If needed, download `operator-sdk`:

- Run `make operator-sdk` to download `operator-sdk` to `./bin` directory.
- Or, download manually by following these [instructions](https://sdk.operatorframework.io/docs/installation/#install-from-github-release).

### Deploy the operator using the operator-sdk


```bash
operator-sdk run bundle --namespace nvidia-network-operator mellanox/network-operator-bundle-1.1.0:latest
```

If needed, kubeconfig file path can be specified:

```bash
operator-sdk run bundle --kubeconfig /path/to/configfile --namespace nvidia-network-operator mellanox/network-operator-bundle-1.1.0:latest
```

Now you should see the `nvidia-network-operator` deployment running in the
`nvidia-network-operator` namespace.

### Deploy the operator from a private registry

When the bundle image and the operator/operand container images are hosted in a private
registry, credentials must be provided in three places:

- The OpenShift global pull secret, so that cluster nodes can pull the operator and operand images.
- A pull secret in the `nvidia-network-operator` namespace, used by the pod that unpacks the bundle.
- The local auth file, used by `operator-sdk` to inspect the bundle image.

The examples below use `nvcr.io` with an NGC API key. You can generate a personal key at
[https://ngc.nvidia.com/setup/api-key](https://ngc.nvidia.com/setup/api-key).

```bash
export NGC_API_KEY=<your NGC API key>
```

#### Add the registry credentials to the OpenShift global pull secret

```bash
oc get secret/pull-secret -n openshift-config --template='{{index .data ".dockerconfigjson" | base64decode}}' > dockerconfigjson
oc registry login --registry="nvcr.io" --auth-basic="\$oauthtoken:${NGC_API_KEY}" --to=dockerconfigjson
oc set data secret/pull-secret -n openshift-config --from-file=.dockerconfigjson=dockerconfigjson
rm -f dockerconfigjson
```

The updated pull secret is rolled out to the nodes by the Machine Config Operator. Wait for
the machine config pools to finish updating:

```bash
oc wait mcp --for condition=updating --all --timeout=30s
oc wait mcp --for condition=updated --all --timeout=600s
```

**NOTE**: The first command may time out if the rollout has already completed; this can be ignored.

#### Create a pull secret in the operator namespace

```bash
oc -n nvidia-network-operator create secret docker-registry ngc-image-secret \
  --docker-server=nvcr.io \
  --docker-username="\$oauthtoken" \
  --docker-password="${NGC_API_KEY}"
```

#### Log in to the registry locally

`operator-sdk` reads registry credentials from `~/.docker/config.json`:

```bash
podman login nvcr.io --authfile ~/.docker/config.json
```

Use `$oauthtoken` as the username and your NGC API key as the password.

#### Run the bundle

Pass the namespace pull secret with `--pull-secret-name`:

```bash
operator-sdk run bundle --namespace nvidia-network-operator \
  --pull-secret-name ngc-image-secret \
  <registry>/<repository>/network-operator-bundle:<version>
```

**NOTE**

To remove the operator when installed via `operator-sdk run`, use:

```bash
operator-sdk cleanup --namespace nvidia-network-operator nvidia-network-operator
```

### Add Environment Variables to Operator Deployment in OpenShift

It is possible to add environment variables to operator deployment in OpenShift
using the deployed operator's `Subscription`.

Get the `Subscription` name:

```
kubectl get subscriptions.operators.coreos.com -n nvidia-network-operator
NAME                                  PACKAGE                   SOURCE                            CHANNEL
nvidia-network-operator-v23-7-0-sub   nvidia-network-operator   nvidia-network-operator-catalog   v23.7.0
```

Edit the `Subscription`, and add a section `spec.config.env` with needed vars and values.
For example:

```
spec:
  config:
    env:
      - name: SKIP_VALIDATIONS
        value: "true"
```