#!/usr/bin/env groovy

library identifier: 'cloudo-shared-lib@master', retriever: modernSCM(
  [$class: 'GitSCMSource',
   remote: 'https://gitlab-master.nvidia.com/cloud-orchestration/cicd/jenkins-confs.git',
   credentialsId: '184e759a-c526-4e1c-b41c-ed7d409efdbb'])

def ref = "${env.gitlabBranch}"
def versionPrefix = 'refs/tags/v'
def tag = ''

if (ref.startsWith(versionPrefix)) {
    tag = ref.substring(versionPrefix.size()-1)
} else if (ref == 'master') {
    tag = 'latest'
}

if (tag == '') {
    return
}

REGISTRY = 'harbor.mellanox.com/cloud-orchestration-dev'
IMAGE_NAME = "${REGISTRY}/network-operator:${tag}"

def image_targets = [
    'image',
    'image-push']

timestamps {
    // This is needed to set the build status to pending befoore running the stages
    gitlabBuilds(builds: image_targets) {
        // This is used to select the worker to run on
        node('universe-builder') {
            checkout scm
            universe.registryLogin()
            image_targets.each { item ->
                universe.runGitlabMake("TAG=${IMAGE_NAME} ${item}", item)
            }
        }
    }
}

