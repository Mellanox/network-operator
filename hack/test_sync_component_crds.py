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

import importlib.util
from pathlib import Path
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("sync-component-crds.py")
SPEC = importlib.util.spec_from_file_location("sync_component_crds", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class SyncComponentCRDsTest(unittest.TestCase):
    def test_match_prefix_uses_complete_basename(self):
        self.assertTrue(MODULE.match_prefix(
            "001-configuration.net.nvidia.com_nicdevices.yaml",
            "configuration.net.nvidia.com_nicdevices.yaml",
        ))
        self.assertTrue(MODULE.match_prefix(
            "0001_spectrumx.nvidia.com_configs.yml",
            "spectrumx.nvidia.com_configs.yml",
        ))
        self.assertFalse(MODULE.match_prefix("001-other-nicdevices.yaml", "nicdevices.yaml"))
        self.assertFalse(MODULE.match_prefix("service-account.yaml", "service-account.yaml"))

    def test_sync_preserves_index_and_license_header(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source_dir = root / "source" / "config/crd/bases"
            target_dir = root / "target" / "manifests/state-component"
            source_dir.mkdir(parents=True)
            target_dir.mkdir(parents=True)
            source = source_dir / "group.example_widgets.yaml"
            target = target_dir / "005-group.example_widgets.yaml"
            source.write_text("---\nkind: CustomResourceDefinition\nspec:\n  version: new\n")
            (source_dir / "kustomization.yaml").write_text("kind: Kustomization\nresources: []\n")
            target.write_text(
                "# Copyright 2026 NVIDIA CORPORATION & AFFILIATES\n"
                "---\nkind: CustomResourceDefinition\nspec:\n  version: old\n"
            )

            args = (root / "source", "config/crd/bases", root / "target", "manifests/state-component")
            self.assertEqual(MODULE.sync_crds(*args), 1)
            self.assertEqual(
                target.read_text(),
                "# Copyright 2026 NVIDIA CORPORATION & AFFILIATES\n" + source.read_text(),
            )
            self.assertEqual(MODULE.sync_crds(*args), 0)

    def test_missing_location_skips_without_inspecting_repositories(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for source, target in (("", ""), ("config/crd/bases", ""), ("", "manifests/state-component")):
                with self.subTest(source=source, target=target):
                    self.assertEqual(MODULE.sync_crds(
                        root / "missing-source", source, root / "missing-target", target,
                    ), 0)

    def test_unmatched_or_ambiguous_target_fails(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source_dir = root / "source"
            target_dir = root / "target"
            source_dir.mkdir()
            target_dir.mkdir()
            (source_dir / "group.example_widgets.yaml").write_text("---\nkind: CustomResourceDefinition\n")
            with self.assertRaisesRegex(ValueError, "found 0"):
                MODULE.sync_crds(source_dir, ".", target_dir, ".")
            (target_dir / "001-group.example_widgets.yaml").write_text("old")
            (target_dir / "002-group.example_widgets.yaml").write_text("old")
            with self.assertRaisesRegex(ValueError, "found 2"):
                MODULE.sync_crds(source_dir, ".", target_dir, ".")

    def test_removed_source_crd_fails_without_modifying_other_targets(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source_dir = root / "source"
            target_dir = root / "target"
            source_dir.mkdir()
            target_dir.mkdir()
            source = source_dir / "group.example_widgets.yaml"
            source.write_text("---\nkind: CustomResourceDefinition\nspec: new\n")
            matched = target_dir / "001-group.example_widgets.yaml"
            matched.write_text("---\nkind: CustomResourceDefinition\nspec: old\n")
            stale = target_dir / "002-group.example_removed.yaml"
            stale.write_text("---\nkind: CustomResourceDefinition\n")
            (target_dir / "003-service-account.yaml").write_text("kind: ServiceAccount\n")

            with self.assertRaisesRegex(ValueError, "has no source"):
                MODULE.sync_crds(source_dir, ".", target_dir, ".")
            self.assertIn("spec: old", matched.read_text())


if __name__ == "__main__":
    unittest.main()
