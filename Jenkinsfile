#!/usr/bin/env groovy

library 'github.com/powerhome/ci-kubed@v6.9.0'

app.build([:]) {
  app.composeBuild(
    appRepo: "image-registry.powerapp.cloud/keess/keess",
  ) { compose ->
    stage('Image Build') {

        try {
            reportBuildState('PENDING')
            withEnv(env) {
                shell "docker build -t ${compose.fullImageName()}"
            }
            pushAll()
            reportBuildState('SUCCESS')
        } catch (e) {
            reportBuildState('FAILURE')
            throw e
        }

        // shell "docker build -t image-registry.powerapp.cloud/keess/keess:${GIT_COMMIT} ."
        // shell "docker push image-registry.powerapp.cloud/keess/keess:${GIT_COMMIT}"
    }
  }
}
