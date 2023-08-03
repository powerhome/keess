#!/usr/bin/env groovy

library 'github.com/powerhome/ci-kubed@v6.10.1'

app.build([:]) {
  app.composeBuild(
    appRepo: "image-registry.powerapp.cloud/keess/keess",
  ) { compose ->
    stage('Image Build') {

        try {
            compose.reportBuildState('PENDING')
            withEnv(compose.environment()) {
                shell "docker build -t ${compose.fullImageName()} ."
            }
            compose.pushAll()
            compose.reportBuildState('SUCCESS')
        } catch (e) { 
            compose.reportBuildState('FAILURE')
            throw e
        }
    }
  }
}
