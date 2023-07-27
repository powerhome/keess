#!/usr/bin/env groovy

node('docker') {
  def imageTag
  def image

  stage('Git Checkout') {
    def scmVars = checkout scm
    imageTag = "${env.BRANCH_NAME.replaceAll('/', '_')}-${scmVars.GIT_COMMIT}-${env.BUILD_ID}"
    image = "image-registry.powerapp.cloud/keess/keess:${imageTag}"
    env.image = image
  }

  stage('Build image') {
    withCredentials([
      usernamePassword(
        credentialsId: 'app-registry-global',
        usernameVariable: 'APP_REGISTRY_USERNAME',
        passwordVariable: 'APP_REGISTRY_PASSWORD'
      )
    ]) {
      // https://issues.jenkins.io/browse/JENKINS-59777
      sh "docker login https://image-registry.powerapp.cloud -u $APP_REGISTRY_USERNAME -p $APP_REGISTRY_PASSWORD"
    }

    shell "docker build -t image-registry.powerapp.cloud/keess/keess:${imageTag} ."
    shell "docker push image-registry.powerapp.cloud/keess/keess:${imageTag}"
  }
}








