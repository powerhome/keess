#!/usr/bin/env groovy

library 'github.com/powerhome/ci-kubed@v8.5.1'

app.build(
  cluster: [:],
  resources: [
    requestCpu: "3",
    limitCpu: "5",
    requestMemory: "2Gi",
    limitMemory: "2Gi",
    requestStorage: '10Gi',
    limitStorage: '10Gi',
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
      compose.bake()
    }
  }
}
