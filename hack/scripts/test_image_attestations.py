# Copyright 2026 NVIDIA CORPORATION & AFFILIATES
# SPDX-License-Identifier: Apache-2.0
"""Offline regression coverage for the image evidence trust boundary."""

import argparse
import base64
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location("attest", Path(__file__).with_name("image-attestations.py"))
attest = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(attest)
INDEX = "sha256:" + "a" * 64
AMD64 = "sha256:" + "b" * 64
ARM64 = "sha256:" + "c" * 64
TAG = "v26.10.0"
SOURCE = attest.source_ref("operator", INDEX)
DEST = "nvcr.io/nvidia/cloud-native/network-operator"
PLATFORMS = {"linux/amd64": AMD64, "linux/arm64": ARM64}
ENV = {
    "GITHUB_REPOSITORY": attest.REPOSITORY,
    "GITHUB_EVENT_NAME": "push",
    "GITHUB_REF": "refs/tags/" + TAG,
    "GITHUB_WORKFLOW_REF": f"{attest.REPOSITORY}/.github/workflows/ci.yaml@refs/tags/{TAG}",
    "GITHUB_SHA": "d" * 40,
    "GITHUB_RUN_ID": "1234",
    "GITHUB_RUN_ATTEMPT": "2",
}


def envelope(predicate, predicate_type, subject=INDEX):
    statement = {"predicateType": predicate_type, "predicate": predicate,
                 "subject": [{"name": SOURCE.split("@")[0], "digest": {"sha256": subject[7:]}}]}
    return json.dumps({"payloadType": "application/vnd.in-toto+json",
                       "payload": base64.b64encode(json.dumps(statement).encode()).decode()})


def sbom(platform, child):
    return {"bomFormat": "CycloneDX", "specVersion": "1.6", "metadata": {"properties": [
        {"name": "network-operator:platform", "value": platform},
        {"name": "network-operator:manifest-digest", "value": child}]}}


class EvidenceTests(unittest.TestCase):
    def setUp(self):
        self.env = patch.dict(os.environ, ENV)
        self.env.start()
        self.addCleanup(self.env.stop)
        self.provenance = attest.provenance("operator", TAG, PLATFORMS)
        self.sboms = [sbom(p, d) for p, d in PLATFORMS.items()]
        self.calls = []
        self.signed_predicates = []
        self.args = argparse.Namespace(variant="operator", digest=INDEX, tag=TAG, destination=DEST)

    def command(self, *args):
        self.calls.append(args)
        if args[:4] == ("docker", "buildx", "imagetools", "inspect"):
            return json.dumps({"manifests": [
                {"digest": d, "platform": dict(zip(("os", "architecture"), p.split("/")))}
                for p, d in PLATFORMS.items()] + [
                    {"digest": "sha256:" + "e" * 64,
                     "platform": {"os": "unknown", "architecture": "unknown"}}]})
        if args[:2] == ("cosign", "verify-attestation"):
            predicate_type = args[args.index("--type") + 1]
            predicates = [self.provenance] if predicate_type == attest.SLSA else self.sboms
            return "\n".join(envelope(p, predicate_type) for p in predicates)
        if args[:2] == ("cosign", "attest"):
            self.signed_predicates.append(json.loads(Path(args[args.index("--predicate") + 1]).read_text()))
        return ""

    def publish(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(attest, "run", side_effect=self.command):
            attest.publish(self.args, Path(directory))

    def assert_no_writes(self):
        self.assertFalse(any(c[:2] in (("cosign", "sign"), ("cosign", "attest")) for c in self.calls))

    def test_publish_preserves_build_provenance_and_both_platform_sboms(self):
        self.publish()
        self.assertEqual(self.signed_predicates, [self.provenance, *self.sboms])
        writes = [c for c in self.calls if c[:2] in (("cosign", "sign"), ("cosign", "attest"))]
        self.assertEqual(len(writes), 4)
        self.assertTrue(all(c[-1] == DEST + "@" + INDEX for c in writes))
        self.assertTrue(all("--new-bundle-format=true" in c for c in writes))
        verify = [c for c in self.calls if c[:2] == ("cosign", "verify")]
        self.assertEqual(verify[0][verify[0].index("--certificate-identity") + 1],
                         attest.WORKFLOW + "@refs/tags/" + TAG)
        self.assertEqual(verify[-1][verify[-1].index("--certificate-identity") + 1], attest.PUBLISHER)
        self.assertFalse(any("--insecure-ignore-tlog" in c or "--key" in c for c in self.calls))

    def test_missing_production_digest_prevents_all_writes(self):
        original = self.command

        def command(*args):
            if args[0] == "docker" and args[-1].startswith(DEST):
                raise subprocess.CalledProcessError(1, args)
            return original(*args)

        self.command = command
        with self.assertRaises(subprocess.CalledProcessError):
            self.publish()
        self.assert_no_writes()

    def test_wrong_certificate_or_missing_attestation_prevents_writes(self):
        original = self.command

        def command(*args):
            if args[:2] == ("cosign", "verify-attestation"):
                raise subprocess.CalledProcessError(1, args)
            return original(*args)

        self.command = command
        with self.assertRaises(subprocess.CalledProcessError):
            self.publish()
        self.assert_no_writes()

    def test_wrong_release_or_variant_prevents_writes(self):
        for field, value in (("ref", "refs/tags/v1.0.0"), ("variant", "coverage")):
            with self.subTest(field=field):
                self.provenance = attest.provenance("operator", TAG, PLATFORMS)
                self.provenance["buildDefinition"]["externalParameters"][field] = value
                with self.assertRaisesRegex(ValueError, "no provenance"):
                    self.publish()
                self.assert_no_writes()

    def test_missing_platform_sbom_prevents_writes(self):
        self.sboms.pop()
        with self.assertRaisesRegex(ValueError, "missing SBOM"):
            self.publish()
        self.assert_no_writes()

    def test_missing_source_commit_prevents_writes(self):
        self.provenance["buildDefinition"]["resolvedDependencies"] = []
        with self.assertRaisesRegex(ValueError, "source commit"):
            self.publish()
        self.assert_no_writes()

    def test_mismatched_child_digest_prevents_writes(self):
        self.sboms[0] = sbom("linux/amd64", ARM64)
        with self.assertRaisesRegex(ValueError, "missing SBOM"):
            self.publish()
        self.assert_no_writes()

    def test_wrong_destination_is_rejected_before_registry_access(self):
        self.args.destination = "nvcr.io/nvidia/cloud-native/unrelated"
        with self.assertRaisesRegex(ValueError, "destination"):
            self.publish()
        self.assertEqual(self.calls, [])

    def test_decoded_subject_is_checked(self):
        with patch.object(attest, "run", return_value=envelope(self.provenance, attest.SLSA, AMD64)):
            with self.assertRaisesRegex(ValueError, "subject digest"):
                attest.verified_predicates(SOURCE, "identity", attest.SLSA)

    def test_duplicate_platform_rejected(self):
        manifest = {"manifests": [{"digest": AMD64, "platform": {"os": "linux", "architecture": "amd64"}}] * 2}
        with patch.object(attest, "run", return_value=json.dumps(manifest)):
            with self.assertRaisesRegex(ValueError, "duplicate"):
                attest.platforms(SOURCE, "operator")

    def test_branch_fork_and_other_workflow_cannot_generate_provenance(self):
        for field, value in (("GITHUB_REF", "refs/heads/master"),
                             ("GITHUB_REPOSITORY", "attacker/network-operator"),
                             ("GITHUB_WORKFLOW_REF", "other.yaml@refs/tags/" + TAG)):
            with self.subTest(field=field), patch.dict(os.environ, {field: value}):
                with self.assertRaises(ValueError):
                    attest.provenance("operator", TAG, PLATFORMS)

    def test_build_scans_immutable_children_and_fails_before_signing_on_scan_error(self):
        original = self.command

        def command(*args):
            if args[0] == "syft":
                self.calls.append(args)
                raise subprocess.CalledProcessError(1, args)
            return original(*args)

        with tempfile.TemporaryDirectory() as directory, patch.object(attest, "run", side_effect=command):
            with self.assertRaises(subprocess.CalledProcessError):
                attest.build(self.args, Path(directory))
        scan = next(c for c in self.calls if c[0] == "syft")
        self.assertEqual(scan[2], "registry:" + SOURCE.split("@")[0] + "@" + AMD64)
        self.assert_no_writes()

    def test_retries_accept_multiple_verified_provenance_envelopes(self):
        output = envelope(self.provenance, attest.SLSA)
        with patch.object(attest, "run", return_value="\n" + output + "\n" + output + "\n"):
            self.assertEqual(attest.verified_predicates(SOURCE, "identity", attest.SLSA),
                             [self.provenance, self.provenance])

    def test_build_all_variants_records_only_the_matching_build_inputs(self):
        for variant in attest.VARIANTS:
            with self.subTest(variant=variant), tempfile.TemporaryDirectory() as directory:
                self.args.variant = variant
                self.calls = []
                self.signed_predicates = []
                bundle = variant.startswith("bundle")
                expected_platforms = {"linux/amd64": INDEX} if bundle else PLATFORMS
                self.provenance = attest.provenance(variant, TAG, expected_platforms)
                self.sboms = [sbom(p, d) for p, d in expected_platforms.items()]
                original = self.command

                def command(*args):
                    if bundle and args[0] == "docker":
                        return json.dumps({"schemaVersion": 2, "config": {}, "layers": []})
                    if args[0] == "syft":
                        self.calls.append(args)
                        return json.dumps({"bomFormat": "CycloneDX", "specVersion": "1.6"})
                    return original(*args)

                with patch.object(attest, "run", side_effect=command):
                    attest.build(self.args, Path(directory))
                self.assertEqual(self.signed_predicates, [self.provenance, *self.sboms])
                scans = [c for c in self.calls if c[0] == "syft"]
                self.assertEqual(len(scans), len(expected_platforms))
                for scan, (platform, child) in zip(scans, sorted(expected_platforms.items())):
                    self.assertEqual(scan[2], "registry:" + attest.source_ref(variant, child))
                    self.assertEqual(scan[scan.index("--platform") + 1], platform)

    def test_production_verification_failure_is_not_reported_as_success(self):
        original = self.command

        def command(*args):
            if args[:2] == ("cosign", "verify") and args[-1].startswith(DEST):
                raise subprocess.CalledProcessError(1, args)
            return original(*args)

        self.command = command
        with self.assertRaises(subprocess.CalledProcessError):
            self.publish()
        self.assertEqual(len(self.signed_predicates), 3)  # Partial publication can be retried.

    def test_cli_rejects_invalid_tag_and_digest_before_external_commands(self):
        for tag, image_digest in (("v26.10.0;touch /tmp/untrusted", INDEX), (TAG, "latest")):
            argv = ["image-attestations.py", "publish", "--variant", "operator",
                    "--tag", tag, "--digest", image_digest, "--destination", DEST]
            with self.subTest(tag=tag), patch.object(sys, "argv", argv), patch.object(attest, "run") as command:
                with self.assertRaises(ValueError):
                    attest.main()
                command.assert_not_called()


if __name__ == "__main__":
    unittest.main()
