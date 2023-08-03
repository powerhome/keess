#!/usr/bin/env groovy

library 'github.com/powerhome/ci-kubed@v6.10.1'

app.build(
  cluster: [:],
  resources: [
    requestCpu: "3",
    limitCpu: "5",
    requestMemory: "2Gi",
    limitMemory: "2Gi",
    requestStorage: '50Gi',
    limitStorage: '50Gi',
  ],
  agentResources: [
    limitCpu: "1.5",
    limitMemory: "1Gi",
    logLevel: "FINEST",
    heapSize: "768m",
  ],
  preferDiskAtLeast: 5,
  requireDiskAtLeast: 1,
  timeout: 200,
) {
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
