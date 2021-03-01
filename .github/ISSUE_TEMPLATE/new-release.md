---
name: New Release
about: Propose a new release
title: Release vx.y.z

---

## Release Checklist
<!--
Please do not remove items from the checklist
-->
- [ ] Network-operator related component versions in Helm chart are up to date. (otherwise, submit PR to update)
  - [ ] node-feature-discovery
  - [ ] SR-IOV Network Operator
  - [ ] Manifest related component default versions
- [ ] Example folder is up to date (otherwise submit PR to update)
- [ ] Update network-operator Helm `Chart.yaml` with the release version (`appVersion`, `version` fields)
  - [ ] Ensure Helm CI is passing on updated Chart.
- [ ] Tag release
- [ ] Create a new github release
  - [ ] Release title: vx.y.z, Release description: Changelog from this issue
  - [ ] Release artifacts for current release
- [ ] Update gh-pages branch
  - [ ] Create Helm package (master branch on release tag commit):
    ```
        > helm package deployments/network-operator
    ```
  - [ ] Place Helm package under gh-pages branch in `release` dir
  - [ ] Update `index.yaml` file under gh-pages branch in `release` dir:
    ```
        > helm repo index . --url https://mellanox.github.io/network-operator/release
    ```
  - [ ] Update gh-pages branch README.md with `deployment/README.md` from master branch (on release tag commit)
  - [ ] Submit PR against `gh-pages` branch:
    ```
        > git add <helm .tgz package> <release/index.yaml> <README.md>
        > git commit --signoff -m "Release Network-Operator vx.y.z"
        > git push ...
    ```
- [ ] Add a link to the release in this issue
- [ ] Close this issue

## Changelog
### New Features
<!--
Describe new features introduced in this release here.
-->
### Bug Fixes
<!--
Describe bugfixes introduced in this release here.
-->
### Known Limitations
<!--
Describe notable known limitations with network-operator (if any) here.
-->
