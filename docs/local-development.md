## Development with minikube and skaffold

The network operator includes tooling to develop locally using `minikube` and `skaffold`.

For more information on `skaffold` [see the official docs](https://skaffold.dev/).
For more information on `minikube` [see the official docs](https://minikube.sigs.k8s.io/docs/).

This development environment is designed to work on Linux amd64 and MacOS on Mac Silicon. It should be extensible for other platforms with some changes to the `minikube` installation.

### Prerequisites:
- `kubectl` CLI
- `docker` CLI - but not necessarily the `docker` engine. This dev setup configures the CLI to use `minikube` as its `docker` engine.
- `docker buildx` CLI plugin. [Installation instructions](https://github.com/docker/buildx?tab=readme-ov-file#manual-download)
- A virtualization driver for minikube. This will be `qemu` for MacOS and `kvm2` for Linux. If this is not installed `minikube` will fail with an error pointing to install instructions.

The `make` targets and scripts will install `minikube` and `skaffold` when running targets.

### Local development with `skaffold`

To spin up a cluster and deploy the network operator run:

`make dev-skaffold`

This will install `skaffold` and `minikube` if needed, spin up a `minikube` cluster called `net-op-dev`, and deploy the network operator in debug mode on the cluster. If there is an error ensure the [prerequisites](#prerequisites), including the virtualization driver, are met.

Other useful `make` targets include `make dev-minikube` to setup the development cluster without skaffold and `make clean-minikube` to tear down the development cluster.

NOTE: If `minikube` with the `kvm` provider is failing with permission errors changing the value of `MINIKUBE_HOME` to a different location could solve the issue. This issue can occur when the user has a network drive for their `$HOME` directory. For example: `MINIKUBE_HOME=/local/path make dev-skaffold`.

### Debugging
This config runs the go debugger `delve` and exposes port `8080` on the remote host that runs the pod.

You can attach to this debugger using your IDE [following the instructions on the `skaffold` documentation.](https://skaffold.dev/docs/workflows/debug/#detailed-debugger-configuration-and-setup).

### Using minikube as a Docker environment

The `skaffold` development setup uses the `minikube` deployment to build docker images and to store them in a registry. This may cause some confusion when running this setup on an installation with multiple docker environments. By design the `minikube` `docker` server is only used by the `dev-skaffold` make target. 

To use the same environment in any shell run:
```bash
eval $(/bin/minikube-XXX -p net-op-dev docker-env)
```
Where XXX is the version of minikube `installed`. Running `docker info | grep net-op-dev` should show the server is running on the minikube VM.

For more information see [the official docs](https://minikube.sigs.k8s.io/docs/tutorials/docker_desktop_replacement/)

## Running skaffold against an existing Kubernetes cluster

`skaffold` can be used independently of `minikube` to deploy the `network-operator` against an existing Kubernetes cluster. This is useful for testing scenarios that require real hardware.

### Prerequisites
NOTE: This list does not include dependencies of the `network-operator` e.g. Node Feature Discovery or cert-manager. These must be installed on the cluster independently where required.

- `skaffold` CLI - https://skaffold.dev/docs/install/#standalone-binary
- An image registry to push images to.
- `kustomize`
- `kubectl`
- Docker
- A running Kubernetes cluster.

### Deploying with Skaffold

Export a registry to push images to:

`export REGISTRY="localhost:5000`

From the root directory of the `network-operator` repository run:

`skaffold dev --default-repo $REGISTRY --trigger manual`

### Remote debugging with Skaffold

Export a registry to push images to:

`export REGISTRY="localhost:5000"`

From the root directory of the `network-operator` repository run:

`skaffold debug --default-repo $REGISTRY`

This config runs the go debugger `delve` and exposes port `8080` on the remote host that runs the pod.

You can attach to this debugger using your IDE [following the instructions on the `skaffold` documentation.](https://skaffold.dev/docs/workflows/debug/#detailed-debugger-configuration-and-setup). 

With VSCode your launch.json should look similar to the below. Note that `remotePath` is set to `""` which is different from the config in the documentation linked above.
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Skaffold Debug",
      "type": "go",
      "request": "attach",
      "mode": "remote",
      "host": "X.X.X.X", // ENTER your hostname here.
      "port": 8080,
      "cwd": "${workspaceFolder}",
      "remotePath": ""
    }
  ]
}
```
