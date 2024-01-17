## Development with skaffold

The network operator includes a `skaffold.yaml` for iterative development with `skaffold`.

For more information on `skaffold` [see the official docs](https://skaffold.dev/).

### Prerequisites
NOTE: This list does not include dependencies of the `network-operator` e.g. Node Feature Discovery or cert-manager. These must be installed on the cluster independently where required.

- Skaffold CLI - https://skaffold.dev/docs/install/#standalone-binary
- An image registry to push images to.
- kustomize
- kubectl
- Docker
- A running Kubernetes cluster.

### Deploying with Skaffold

Export a registry to push images to:

`export REGISTRY="localhost:5000`

From the root directory of the `network-operator` repository run:

`skaffold dev --default-repo $REGISTRY --trigger manual`

### Remote debugging with Skaffold

Export a registry to push images to:

`export REGISTRY="localhost:5000`

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
