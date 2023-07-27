#!/usr/bin/env groovy

library 'github.com/powerhome/ci-kubed@v6.9.0'

app.build([:]) {
  app.dockerStage('Container Build') {
    shell "docker build -t image-registry.powerapp.cloud/keess/keess:${APP_IMAGE_TAG} ."
    shell "docker push image-registry.powerapp.cloud/keess/keess:${APP_IMAGE_TAG}"
  }
}
