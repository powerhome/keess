#!/usr/bin/env groovy

library 'github.com/powerhome/ci-kubed@1c81d877a05417860bca4d369e088bca18fe640b'

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
