#!/usr/bin/env python3
# Copyright 2026 NVIDIA CORPORATION & AFFILIATES
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

import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]


class DocaDriverManifestTest(unittest.TestCase):
    def test_check_selects_driver_release_manifest(self):
        cases = (
            ("doca3.6.0-26.10-070000-1", "26.10"),
            ("doca3.6.1-26.10-011000-0", "26.10"),
            ("doca3.3.3-26.01-2.0.0.0.903-0", "26.01"),
            ("doca3.6.12-26.10-011000-0", "26.10"),
        )
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            # Stub only external tools: exercise the real Makefile recipe without
            # downloading a manifest or querying a container registry.
            commands = {
                "yq": '#!/bin/sh\nprintf "%s\\n" "$TEST_DRIVER_VERSION"\n',
                "wget": '#!/bin/sh\nprintf "%s\\n" "$@" > "$TEST_DOWNLOAD_ARGS"\n',
                "go": '#!/bin/sh\nif [ "$1" = run ]; then printf "%s\\n" "$@" > "$TEST_GO_ARGS"; fi\n',
            }
            for name, content in commands.items():
                command = directory / name
                command.write_text(content)
                command.chmod(0o755)
            download_args = directory / "download-args"
            go_args = directory / "go-args"
            matrix = directory / "doca-driver-matrix.yaml"
            for version, driver_release in cases:
                with self.subTest(version=version):
                    download_args.unlink(missing_ok=True)
                    go_args.unlink(missing_ok=True)
                    result = subprocess.run(
                        ["make", "--no-print-directory", "check-doca-drivers",
                         f"HACKTMPDIR={directory}",
                         "DOCA_DRIVER_RELEASE_URL=https://example.invalid/manifests/"],
                        cwd=ROOT,
                        env={**os.environ, "PATH": f"{directory}:{os.environ['PATH']}",
                             "TEST_DRIVER_VERSION": version,
                             "TEST_DOWNLOAD_ARGS": str(download_args),
                             "TEST_GO_ARGS": str(go_args)},
                        capture_output=True, text=True, check=False,
                    )
                    self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                    self.assertEqual(download_args.read_text().splitlines(), [
                        f"https://example.invalid/manifests/{driver_release}.yaml",
                        "-O", str(matrix),
                    ])
                    self.assertEqual(go_args.read_text().splitlines(), [
                        "run", "release.go", "--doca-driver-check",
                        "--doca-driver-matrix", str(matrix),
                    ])


if __name__ == "__main__":
    unittest.main()
