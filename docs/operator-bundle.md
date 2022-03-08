# Operator build and deployment


## Building the operator bundle

For development and testing purposes it may be beneficial to build the operator bundle.

The template for the CSV is located [here](config/manifests/bases/nvidia-network-operator.clusterserviceversion.yaml).

Build the bundle:

```bash
DEFAULT_CHANNEL=v1.1.0 CHANNELS=v1.1.0 VERSION=1.1.0 TAG=nvcr.io/nvidia/cloud-native/network-operator@sha256:17afa53f1cf3733c8d0cd282c565975ed5de3124dfc2b7c485ad12c97e51c251 make bundle
```

Build the bundle image:

```bash
BUNDLE_IMG=mellanox/network-operator-bundle-1.1.0 make bundle-build
```

Push the bundle image:

```bash
BUNDLE_IMG=mellanox/network-operator-bundle-1.1.0 make bundle-push
```

**NOTE**

It is recommended to use sha256 instead of tag.
Here how to obtain the digest:

```bash
skopeo inspect docker://nvcr.io/nvidia/cloud-native/network-operator:v1.1.0 | jq .Digest
"sha256:17afa53f1cf3733c8d0cd282c565975ed5de3124dfc2b7c485ad12c97e51c251"
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

Deploy the operator using the operator-sdk:

```bash
operator-sdk run bundle --namespace nvidia-network-operator mellanox/network-operator-bundle-1.1.0
```

Now you should see the `nvidia-network-operator` deployment running in the
`nvidia-network-operator` namespace.

**NOTE**

To remove the operator when installed via `operator-sdk run`, use:

```bash
operator-sdk cleanup --namespace nvidia-network-operator nvidia-network-operator
```

