# Keess End-to-End Tests

This directory contains end-to-end (e2e) tests for the Keess synchronization service using Ginkgo v2 and Gomega testing frameworks.

## Prerequisites

These tests are organized in a way to reuse resources that can be used for manual testing and experimentation of the features, to foster this experimentation, and to avoid duplication of setup code.

So the tests use the clusters brought up by the Makefile, and also use the example files provided in the repo. That also makes the tests Go code cleaner.

The trade-off is that we have a few dependencies to run the tests:

- **Kind Clusters**: Two local Kubernetes clusters (`make setup-local-clusters`)
- **Keess Service**: The synchronization service must be running (`make run`)
- **Test Resources**: Example YAML files in the `../examples/` directory
- **Install Ginkgo**: `go install github.com/onsi/ginkgo/v2/ginkgo` (your go distribution binary folder must be on your path for the tests to run)

## Running

We will use [kind](https://kind.sigs.k8s.io/) for this. First, create and configure local test clusters with:

```shell
make setup-local-clusters-with-keess
```

> [!NOTE]
> This will create clusters in "PAC-v2 style": with recent Kubernetes version, Cilium CNI, AND Keess running on those.
>
> For running the full e2e test suite you will need that because you need Keess running on both the source and destination clusters.
>
> You can also just create the clusters with `make setup-local-clusters` and run Keess locally on your machine with `make run` or `make local-docker-run`.
>
> To create clusters for testing Keess on PAC-V1 (no Cilium, old Kubernetes version), use `create-local-clusters-pac-v1`.

Run all tests with:

```shell
make tests-all
```

See `make help` for individual test options.

### Updating Keess on local clusters

To update Keess on the running local clusters, just run:

 `make install-keess`

It build the new image, install it on the kind clusters, and restart the running pods.

### Investigating cluster state

If you want to investigate the cluster you can do it by:

```shell
kubectl cluster-info --context kind-source-cluster --kubeconfig localTestKubeconfig
kubectl cluster-info --context kind-destination-cluster --kubeconfig localTestKubeconfig
```

### Cleaning up

Once we are done with the test and don't need the local clusters anymore you can delete them with

```shell
make delete-local-clusters
```

## Running selected tests with Ginkgo

Labels area attached to ginkgo tests, and you can filter tests based on those:

```sh
cd tests/e2e
ginkgo -v --label-filter 'secret'
```

Run `ginkgo labels` to list available labels.

You can also exclude tests with `!`: `ginkgo -v --label-filter '!secret'`.

## Older Python e2e tests

They should eventually be fully replaced by Go tests. They are important because they
cover the namespace sync functionality, while Go e2e tests cover cluster sync.

They will need the local clusters and keess to be up, just as the Go e2e tests.

```shell
make tests-python-e2e
```

## Source code organization and style

### Ginkgo and BDD

The tests on tests/e2e folder use [Ginkgo](https://onsi.github.io/ginkgo) and follow Behavior-Driven Development (BDD) patterns:

```go
Describe("Resource Sync", func() {
    Context("On Cluster mode", func() {
        When("an annotated resource is created", func() {
            It("should be synced to destination cluster", func() {
                // Test implementation
            })
        })
    })
})
```

### Test Suite (`e2e_suite_test.go`)

The main test suite file that provides:

- **Cluster Setup**: Configures connections to source and destination Kind clusters
- **Helper Functions**: Utilities for namespace management, resource application, and cleanup
- **Custom Matchers**: Gomega matchers for validating synchronized resources
- **Constants**: Shared configuration like timeouts, polling intervals, and cluster contexts

### Other test files

Usually one per feature of Keess (resources that is supports syncing).
