# Release image signatures and attestations

Release-tag CI generates keyless signatures, CycloneDX 1.6 SBOM attestations,
and signed [SLSA v1 provenance](https://slsa.dev/spec/v1.2/build-provenance)
for the operator, coverage operator, OCP bundle, and coverage bundle images.
Branch and pull-request builds do not run these signing steps. This applies to
new release tags containing this workflow; it does not retrofit historical images.

The first delivery claims **SLSA Build L1 (provenance exists)** with signatures.
The build job itself constructs the provenance and can obtain signing identity
tokens. This implementation does not establish the platform-controlled provenance
or hardened isolation needed for L2/L3. Signing after promotion does not raise the
build level. The namespaced provenance extension
`https://github.com/Mellanox/network-operator/assurance` records `slsaBuildLevel: 1`.

Helm chart signatures, images built by other repositories (including DOCA),
hardened builders, and integration into the complete production promotion system
remain separate work under [#2555](https://github.com/Mellanox/network-operator/issues/2555).

## Build and production publication

1. `ci.yaml` obtains immutable image digests from Docker Buildx outputs, or the
   local pushed image's `RepoDigests` for bundles. It generates and verifies
   signed evidence at `nvcr.io/nvstaging/mellanox/<image>@sha256:...`.
2. The existing release process promotes the image, preserving its digest.
   This change does not promote images or alter production release tags.
3. A release owner dispatches **Publish production image attestations** on
   `master`, providing the original release tag, promoted digest, variant, and
   production repository. For operator and coverage images the only accepted
   destination is `nvcr.io/nvidia/cloud-native/network-operator`.
   For bundles, select the repository actually used by the existing promotion;
   the workflow supports the `nvidia/cloud-native` and `nvidia/mellanox` namespaces.
   This does not provision either repository or decide which should be used.
4. Before writing, the job checks the production manifest exists at the same
   digest, verifies the staging evidence against the exact release-tag CI
   identity, and checks the release, variant, platform digests and complete SBOM
   set. It then signs the production digest and republishes the verified
   predicates with the production subject. The original build commit, builder,
   invocation and assurance level are retained. It verifies the production
   signatures and attestations before reporting success.

The production workflow requires an `image-attestations-production` GitHub
environment with release-owner reviewers and a deployment-branch rule permitting
only `master`. Configure its `NVCR_ATTESTATION_TOKEN` secret to read staging
images/evidence and write evidence to the selected production repository.
These environment settings and credentials must be provisioned before use.
The staging build continues to use its existing NGC credential.

Cosign v3.1.4 is pinned and explicitly uses OCI Sigstore bundles, Fulcio and the
public Rekor transparency log. The workflow does not overwrite NGC's legacy
key-based `.sig` artifacts. Publication needs registry support for OCI referrers
or Cosign's tag fallback. Confirm this, NGC permissions, public read access and
the exact production signer identity with a controlled release before enabling
consumer enforcement. A failure may leave partial evidence; retry the same
digest and consider publication complete only after all verification passes.
Production jobs for the same repository and digest are serialized.

## Consumer verification

Use Cosign v3.1.4. Obtain the expected digest and release tag from your trusted
release configuration. Verify against the **production index digest** for a
multi-platform operator image, not an architecture-specific child digest.
Evidence is attached to the index; there is one SBOM per platform, each scanned
from that platform's immutable manifest. The SBOM metadata properties
`network-operator:platform` and `network-operator:manifest-digest` identify its
scope. A bundle has one SBOM; a scratch bundle may legitimately have no packages.
The bundle SBOM describes the bundle filesystem, not the referenced operator or
its separately built components.

```bash
image='nvcr.io/nvidia/cloud-native/network-operator@sha256:<release-index-digest>'
identity='https://github.com/Mellanox/network-operator/.github/workflows/publish-image-attestations.yaml@refs/heads/master'
issuer='https://token.actions.githubusercontent.com'

cosign verify --new-bundle-format=true \
  --certificate-identity "$identity" --certificate-oidc-issuer "$issuer" "$image"

cosign verify-attestation --new-bundle-format=true \
  --certificate-identity "$identity" --certificate-oidc-issuer "$issuer" \
  --type https://slsa.dev/provenance/v1 "$image" > provenance.dsse.jsonl

cosign verify-attestation --new-bundle-format=true \
  --certificate-identity "$identity" --certificate-oidc-issuer "$issuer" \
  --type https://cyclonedx.org/bom "$image" > sbom.dsse.jsonl
```

Do not disable transparency-log, certificate or subject-digest checks. Use the
full SLSA predicate URI to avoid version-dependent shorthand aliases. These
commands authenticate the signer and subject; consumers must also inspect the
verified predicates and enforce their expected release, source commit, variant,
builder and platform coverage. For example, decode verified output with:

```bash
jq -r '.payload | @base64d' provenance.dsse.jsonl | jq .
jq -r '.payload | @base64d' sbom.dsse.jsonl | jq .
```

Require `buildDefinition.externalParameters.repository` to be
`Mellanox/network-operator`, `ref` to be the intended `refs/tags/<release>`, and
`variant` to match `operator`, `coverage`, `bundle` or `bundle-coverage`.
Check the source commit in `resolvedDependencies` against your expected release
commit and the builder against the CI workflow below. For operator images require
both `linux/amd64` and `linux/arm64` SBOMs and match their manifest-digest properties
to the index's children. Successful verification of one SBOM alone does not prove
complete multi-platform inventory. Reruns may yield multiple valid attestations.

## Build type v1

Build type URI:
`https://github.com/Mellanox/network-operator/blob/master/docs/image-verification.md#build-type-v1`.
Builder ID:
`https://github.com/Mellanox/network-operator/.github/workflows/ci.yaml`.

The source commit and tag identify the workflow and Dockerfiles to inspect.
`externalParameters` contains the repository, source tag ref, and image variant.
`operator` and `coverage` execute the multi-platform Dockerfile build in
`docker-build-push`; coverage additionally sets `GO_COVERAGE=1`.
`bundle` executes `make bundle bundle-build bundle-push` in `ocp-bundle`;
`bundle-coverage` executes the corresponding coverage targets in
`ocp-bundle-coverage`. Bundle manifests are generated by those targets before
the image build. `resolvedDependencies` records the source Git commit, not an
exhaustive set of base images, tool downloads, Go modules or generated inputs.
The invocation URL identifies the run and attempt whose logs record execution.

## Validation

Offline tests exercise missing production images, wrong source identity,
release/variant mismatches, missing or incorrect platform SBOMs, digest mismatch,
scan failures, duplicate platforms, and preservation of the original provenance.

```bash
python3 -m unittest discover -s hack/scripts -p 'test_image_attestations.py' -v
```

They mock registry and signing services; they do not establish NGC compatibility,
OIDC/Fulcio/Rekor operation, production permissions or release-owner procedures.
Those checks remain required before this workflow is qualified for production.
