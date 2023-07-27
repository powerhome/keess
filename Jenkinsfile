#!/usr/bin/env groovy

app.build([:]) {
  app.dockerStage('Container Build') {
    shell "docker build -t image-registry.powerapp.cloud/keess/keess:${APP_IMAGE_TAG} ."
    shell "docker push image-registry.powerapp.cloud/keess/keess:${APP_IMAGE_TAG}"
  }
}
