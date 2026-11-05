#!/usr/bin/env python3
# Copyright 2026 NVIDIA CORPORATION & AFFILIATES
# SPDX-License-Identifier: Apache-2.0
"""Build and publish digest-bound image evidence. See docs/image-verification.md."""

import argparse
import base64
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile

REPOSITORY = "Mellanox/network-operator"
WORKFLOW = f"https://github.com/{REPOSITORY}/.github/workflows/ci.yaml"
PUBLISHER = f"https://github.com/{REPOSITORY}/.github/workflows/publish-image-attestations.yaml@refs/heads/master"
ISSUER = "https://token.actions.githubusercontent.com"
SLSA = "https://slsa.dev/provenance/v1"
CYCLONEDX = "https://cyclonedx.org/bom"
BUILD_TYPE = f"https://github.com/{REPOSITORY}/blob/master/docs/image-verification.md#build-type-v1"
ASSURANCE = f"https://github.com/{REPOSITORY}/assurance"
VARIANTS = ("operator", "coverage", "bundle", "bundle-coverage")


def require(condition, message):
    if not condition:
        raise ValueError(message)


def run(*args):
    # Keep stderr visible (including on failure); never invoke a shell.
    return subprocess.run(args, check=True, stdout=subprocess.PIPE, text=True).stdout


def digest(value):
    require(re.fullmatch(r"sha256:[0-9a-f]{64}", value), "expected a sha256 digest")
    return value


def image_name(variant):
    return "network-operator-bundle" if variant.startswith("bundle") else "network-operator"


def source_ref(variant, image_digest):
    return f"nvcr.io/nvstaging/mellanox/{image_name(variant)}@{digest(image_digest)}"


def platforms(image, variant):
    manifest = json.loads(run("docker", "buildx", "imagetools", "inspect", "--raw", image))
    if variant.startswith("bundle"):
        require("manifests" not in manifest, "expected a single-platform bundle image")
        return {"linux/amd64": image.split("@")[1]}
    result = {}
    for child in manifest.get("manifests", []):
        platform = child.get("platform", {})
        if platform.get("os") == "unknown" and platform.get("architecture") == "unknown":
            continue  # BuildKit's embedded attestations are not root filesystems.
        name = f"{platform.get('os')}/{platform.get('architecture')}"
        require(name in ("linux/amd64", "linux/arm64") and name not in result,
                "unexpected or duplicate image platform")
        result[name] = digest(child["digest"])
    require(set(result) == {"linux/amd64", "linux/arm64"}, "both image platforms are required")
    return result


def provenance(variant, tag, image_platforms):
    ref = f"refs/tags/{tag}"
    require(os.environ["GITHUB_REPOSITORY"] == REPOSITORY, "unexpected build repository")
    require(os.environ["GITHUB_EVENT_NAME"] == "push" and os.environ["GITHUB_REF"] == ref,
            "only release tag pushes may generate build evidence")
    require(os.environ["GITHUB_WORKFLOW_REF"] == f"{REPOSITORY}/.github/workflows/ci.yaml@{ref}",
            "unexpected build workflow")
    sha = os.environ["GITHUB_SHA"]
    require(re.fullmatch(r"[0-9a-f]{40}", sha), "invalid source commit")
    return {
        "buildDefinition": {
            "buildType": BUILD_TYPE,
            "externalParameters": {"repository": REPOSITORY, "ref": ref, "variant": variant},
            "resolvedDependencies": [{"uri": f"git+https://github.com/{REPOSITORY}@{ref}",
                                      "digest": {"gitCommit": sha}}],
        },
        "runDetails": {
            "builder": {"id": WORKFLOW},
            "metadata": {"invocationId": f"https://github.com/{REPOSITORY}/actions/runs/"
                         f"{os.environ['GITHUB_RUN_ID']}/attempts/{os.environ['GITHUB_RUN_ATTEMPT']}"},
        },
        ASSURANCE: {"slsaBuildLevel": 1, "platforms": image_platforms},
    }


def sign(image, predicates, directory):
    # OCI Sigstore bundles coexist with existing NGC key-based .sig attachments.
    for index, (predicate_type, predicate) in enumerate(predicates):
        path = directory / f"predicate-{index}.json"
        path.write_text(json.dumps(predicate))
        run("cosign", "attest", "--yes", "--new-bundle-format=true", "--type", predicate_type,
            "--predicate", str(path), image)
    # Sign last: a signature is never our indication that evidence generation succeeded.
    run("cosign", "sign", "--yes", "--new-bundle-format=true", image)


def verified_predicates(image, identity, predicate_type):
    output = run("cosign", "verify-attestation", "--new-bundle-format=true",
                 "--certificate-identity", identity, "--certificate-oidc-issuer", ISSUER,
                 "--type", predicate_type, image)
    # Cosign emits one DSSE envelope per verified attestation, not a JSON array.
    decoder = json.JSONDecoder()
    result = []
    while output.strip():
        envelope, end = decoder.raw_decode(output.lstrip())
        output = output.lstrip()[end:]
        require(envelope["payloadType"] == "application/vnd.in-toto+json", "invalid DSSE payload type")
        statement = json.loads(base64.b64decode(envelope["payload"], validate=True))
        require(statement["predicateType"] == predicate_type, "unexpected predicate type")
        require(any(s.get("digest", {}).get("sha256") == image.split("@sha256:")[1]
                    for s in statement["subject"]), "attestation subject digest mismatch")
        result.append(statement["predicate"])
    require(result, "no verified predicates")
    return result


def verify(image, identity, variant, tag, image_platforms):
    run("cosign", "verify", "--new-bundle-format=true", "--certificate-identity", identity,
        "--certificate-oidc-issuer", ISSUER, image)
    provenances = verified_predicates(image, identity, SLSA)
    matching = [p for p in provenances if
                p.get("buildDefinition", {}).get("buildType") == BUILD_TYPE and
                p["buildDefinition"].get("externalParameters") == {
                    "repository": REPOSITORY, "ref": f"refs/tags/{tag}", "variant": variant} and
                p.get("runDetails", {}).get("builder", {}).get("id") == WORKFLOW and
                p.get(ASSURANCE) == {"slsaBuildLevel": 1, "platforms": image_platforms}]
    require(matching, "no provenance for the requested release, variant and platforms")
    provenance = matching[-1]
    dependencies = provenance["buildDefinition"].get("resolvedDependencies", [])
    require(any(d.get("uri") == f"git+https://github.com/{REPOSITORY}@refs/tags/{tag}" and
                re.fullmatch(r"[0-9a-f]{40}", d.get("digest", {}).get("gitCommit", ""))
                for d in dependencies), "missing source commit in provenance")
    invocation = provenance["runDetails"].get("metadata", {}).get("invocationId", "")
    require(re.fullmatch(rf"https://github.com/{REPOSITORY}/actions/runs/[0-9]+/attempts/[0-9]+", invocation),
            "missing build invocation in provenance")
    sboms = verified_predicates(image, identity, CYCLONEDX)
    selected = []
    for platform, child_digest in sorted(image_platforms.items()):
        matches = [s for s in sboms if s.get("bomFormat") == "CycloneDX" and
                   {p["name"]: p["value"] for p in s.get("metadata", {}).get("properties", [])}.items()
                   >= {"network-operator:platform": platform,
                       "network-operator:manifest-digest": child_digest}.items()]
        require(matches, f"missing SBOM for {platform}@{child_digest}")
        selected.append((CYCLONEDX, matches[-1]))
    # Re-runs may leave several valid attestations. Retain a complete verified set.
    return [(SLSA, provenance), *selected]


def build(args, directory):
    image = source_ref(args.variant, args.digest)
    image_platforms = platforms(image, args.variant)
    predicates = [(SLSA, provenance(args.variant, args.tag, image_platforms))]
    for platform, child_digest in sorted(image_platforms.items()):
        child = image.split("@")[0] + "@" + child_digest
        sbom = json.loads(run("syft", "scan", "registry:" + child, "--platform", platform,
                             "--output", "cyclonedx-json@1.6"))
        require(sbom.get("bomFormat") == "CycloneDX" and sbom.get("specVersion") == "1.6",
                "Syft did not produce the expected CycloneDX format")
        sbom.setdefault("metadata", {}).setdefault("properties", []).extend([
            {"name": "network-operator:platform", "value": platform},
            {"name": "network-operator:manifest-digest", "value": child_digest},
        ])
        predicates.append((CYCLONEDX, sbom))
    sign(image, predicates, directory)
    verify(image, f"{WORKFLOW}@refs/tags/{args.tag}", args.variant, args.tag, image_platforms)


def publish(args, directory):
    # The operator destination is fixed. Bundle destination is a separate release-owner choice.
    allowed = {"operator": {"nvcr.io/nvidia/cloud-native/network-operator"},
               "coverage": {"nvcr.io/nvidia/cloud-native/network-operator"},
               "bundle": {"nvcr.io/nvidia/cloud-native/network-operator-bundle",
                          "nvcr.io/nvidia/mellanox/network-operator-bundle"}}
    require(args.destination in allowed["bundle" if args.variant.startswith("bundle") else args.variant],
            "destination does not match the image variant")
    source = source_ref(args.variant, args.digest)
    destination = f"{args.destination}@{args.digest}"
    image_platforms = platforms(source, args.variant)
    # Check promotion before any write: the already-published manifest must exist by the SAME digest.
    require(platforms(destination, args.variant) == image_platforms, "production platform mismatch")
    predicates = verify(source, f"{WORKFLOW}@refs/tags/{args.tag}", args.variant, args.tag, image_platforms)
    sign(destination, predicates, directory)
    verify(destination, PUBLISHER, args.variant, args.tag, image_platforms)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=("build", "publish"))
    parser.add_argument("--variant", choices=VARIANTS, required=True)
    parser.add_argument("--digest", required=True)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--destination")
    args = parser.parse_args()
    digest(args.digest)
    require(re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?", args.tag), "invalid release tag")
    with tempfile.TemporaryDirectory(prefix="image-attestations-") as directory:
        {"build": build, "publish": publish}[args.mode](args, Path(directory))


if __name__ == "__main__":
    main()
