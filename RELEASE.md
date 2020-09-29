# Release process

1. Create a helm package for the release
```
# On master branch
$ helm package deployments/network-operator
```
2. checkout `gh-pages` branch
```
$ git checkout gh-pages
```
3. move helm package tarball into release folder
```
$ mv <helm package tarball> release/
```
4. create `index.yaml` file
```
$ helm repo index . --url https://mellanox.github.io/network-operator/release
```
5. commit the changes
```
$ cd release
$ git add <helm tgz package> ./index.yaml
$ git commit --signoff -m 'Release version xxx'
```
