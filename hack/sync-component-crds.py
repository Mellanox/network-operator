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

"""Copy component CRDs into their existing, numbered state manifests."""

import argparse
from pathlib import Path
import re


INDEXED_NAME = re.compile(r"^[0-9]+[-_](.+\.ya?ml)$")
DOCUMENT_START = re.compile(r"^---[ \t]*\r?$", re.MULTILINE)
CRD_KIND = re.compile(r"^kind:[ \t]*CustomResourceDefinition[ \t]*\r?$", re.MULTILINE)


def match_prefix(manifest_filename: str, original_filename: str) -> bool:
    """Match the complete CRD basename after the manifest's numeric index."""
    match = INDEXED_NAME.fullmatch(manifest_filename)
    return match is not None and match.group(1) == original_filename


def preserve_target_header(source: str, target: str) -> str:
    """Keep the target's license comments while replacing its YAML document."""
    target_start = DOCUMENT_START.search(target)
    if target_start is None:
        return source

    header = target[: target_start.start()]
    if not all(not line.strip() or line.lstrip().startswith("#") for line in header.splitlines()):
        return source

    source_start = DOCUMENT_START.search(source)
    document = source[source_start.start() :] if source_start else source
    return header + document


def sync_crds(source_repo: Path, source_location: str, target_repo: Path, target_location: str) -> int:
    if not source_location or not target_location:
        return 0

    source_dir = source_repo / source_location
    target_dir = target_repo / target_location
    if not source_dir.is_dir():
        raise ValueError(f"CRD source directory does not exist: {source_dir}")
    if not target_dir.is_dir():
        raise ValueError(f"CRD target directory does not exist: {target_dir}")

    sources = sorted(
        path for path in source_dir.iterdir()
        if path.is_file() and path.suffix in (".yaml", ".yml") and CRD_KIND.search(path.read_text())
    )
    if not sources:
        raise ValueError(f"No CustomResourceDefinition YAML files found in {source_dir}")

    targets = [path for path in target_dir.iterdir() if path.is_file()]
    matches_by_source = {}
    for source in sources:
        matches = [target for target in targets if match_prefix(target.name, source.name)]
        if len(matches) != 1:
            raise ValueError(f"Expected one indexed manifest for {source}, found {len(matches)}: {matches}")
        matches_by_source[source.name] = matches[0]

    source_names = set(matches_by_source)
    for target in targets:
        match = INDEXED_NAME.fullmatch(target.name)
        if match and CRD_KIND.search(target.read_text()) and match.group(1) not in source_names:
            raise ValueError(f"Indexed CRD manifest has no source in {source_dir}: {target}")

    updated = 0
    for source in sources:
        target = matches_by_source[source.name]
        original = target.read_text()
        content = preserve_target_header(source.read_text(), original)
        if content != original:
            target.write_text(content)
            updated += 1
            print(f"Updated {target} from {source}")
    return updated


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source_repo", type=Path)
    parser.add_argument("source_location")
    parser.add_argument("target_repo", type=Path)
    parser.add_argument("target_location")
    args = parser.parse_args()
    updated = sync_crds(args.source_repo, args.source_location, args.target_repo, args.target_location)
    print(f"Synced {updated} CRD manifests")


if __name__ == "__main__":
    main()
