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
    - [ ] Verify that new versions of operator, configdaemon and webhooks components are compatible
    - [ ] Use latest releases of sriovcni and ibsriovcni
  - [ ] Manifest related component default versions
- [ ] Example folder is up to date (otherwise submit PR to update)
- [ ] Update network-operator Helm `Chart.yaml` with the release version (`appVersion`, `version` fields)
  ```
       > ./scripts/releases/prepare-release.sh v0.1.2 "Jane Doe <jane.doe@example.com>"
  ```
  - [ ] Ensure Helm CI is passing on updated Chart.
- [ ] Create a new github release
  - [ ] Release title: vx.y.z, Release description: Changelog from this issue
  - [ ] Release artifacts for current release
- [ ] Update gh-pages branch using the script
  ```
       > ./scripts/releases/update-gh-pages.sh -p <remote_fork> network-operator-0.1.2.tgz
  ```
  Or do it manually:
    - [ ] Create Helm package (master branch on release tag commit):
      ```
        > helm package deployment/network-operator
      ```
    - [ ] Place Helm package under gh-pages branch in `release` dir
    - [ ] Update `index.yaml` file under gh-pages branch in `release` dir:
      ```
        > # assuming we are under release dir
        > mkdir tmpdir; cp <helm-package.tgz> ./tmpdir
        > helm repo index ./tmpdir --url https://mellanox.github.io/network-operator/release --merge ./index.yaml
        > mv -f ./tmpdir/index.yaml ./; rm -rf ./tmpdir
      ```
    - [ ] Push to remote:
      ```
        > git add <helm .tgz package> <release/index.yaml> <README.md>
        > git commit -s -m "Release Network-Operator vx.y.z"
        > git push ...
      ```
  After that:
  - [ ] Update gh-pages branch README.md with `deployment/README.md` from master branch (on release tag commit)
  - [ ] Submit PR against `gh-pages` branch
- [ ] Add a link to the release in this issue
- [ ] Verify new image is published to the registry
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
