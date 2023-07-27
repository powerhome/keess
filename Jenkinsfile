#!/usr/bin/env groovy

library 'github.com/powerhome/ci-kubed@v6.9.0'

app.build([:]) {
  app.composeBuild(
    appRepo: "image-registry.powerapp.cloud/keess/keess",
  ) { compose ->    
    stage('Run image Build') {
      shell "docker build -t image-registry.powerapp.cloud/keess/keess:${APP_IMAGE_TAG} ."
    }
    stage('Run image Push') {
      shell "docker push image-registry.powerapp.cloud/keess/keess:${APP_IMAGE_TAG}"
    }    
  }
}
