
# Keess: Kubernetes Secrets and ConfigMaps Synchronization

Keess (Keep Stuff Synchronized) is a versatile command-line tool designed to synchronize secrets and configmaps across different namespaces and Kubernetes clusters. Built with simplicity and efficiency in mind, it ensures that your Kubernetes environments are consistently updated, secure, and easy to manage.

## Features

- **Cross-Namespace Synchronization**: Effortlessly sync secrets and configmaps across multiple namespaces within a single Kubernetes cluster.
- **Inter-Cluster Synchronization**: Extend your synchronization capabilities to multiple clusters, keeping your configurations consistent across different environments.
- **Secure and Reliable**: Implements robust mechanisms to securely transfer sensitive information, ensuring data integrity and confidentiality.
- **Automation**: Automates the synchronization process, reducing manual overhead and minimizing human error.
- **Customizable**: Offers flexible command line options and Kubernetes annotations to tailor the synchronization process to your specific needs.
- **Efficient Monitoring**: Provides detailed logs for tracking operations and auditing changes.

## Getting Started

### Prerequisites

- Kubernetes cluster setup
- kubectl installed and configured
- Helm (optional, for Helm chart deployment)

### Installation

Refer to the previous section on installing Keess via binaries, source, or Helm.

### Configuration

#### Using Configuration Files

Create a `.keess.yaml` configuration file as previously described or specify the path using the `--config` flag.

#### Using Command Line Flags

Keess supports various command line flags for on-the-fly configuration:

```shell
./keess run --logLevel debug --localCluster my-cluster --kubeConfigPath /path/to/kubeconfig
```

For a full list of available flags, use:

```shell
./keess --help
```

### Configuring Synchronization

Keess uses Kubernetes labels and annotations to manage synchronization of Secrets and ConfigMaps.

#### Enable Synchronization

Add a label to your Secret or ConfigMap to indicate the synchronization type:

- For namespace synchronization: `keess.powerhrg.com/sync: namespace`
- For cluster synchronization: `keess.powerhrg.com/sync: cluster`

#### Namespace Synchronization

Configure which namespaces to synchronize with:

- All namespaces: `keess.powerhrg.com/namespaces-names: all`
- Specific namespaces: `keess.powerhrg.com/namespaces-names: namespacea, namespaceb`
- Based on labels: `keess.powerhrg.com/namespace-label: keess.powerhrg.com/sync="true"`

#### Cluster Synchronization

Specify the remote clusters for synchronization: `keess.powerhrg.com/clusters: clustera, clusterb`

## Contributing

Contributions are welcome! Please refer to our [Contributing Guidelines](CONTRIBUTING.md) for more information.

## Support

If you encounter any issues or have questions, please file an issue on the [GitHub Issues page](https://github.com/your-repo/keess/issues).

## License

Keess is open-source software licensed under the MIT license. See the [LICENSE](LICENSE) file for details.

## Local testing
We will use [kind](https://kind.sigs.k8s.io/) for this

First of all, create 2 clusters:
```
make create-local-clusters
```

Now build and run the application locally pointing to these new clusters:
```
make docker-build local-docker-run
```

To execute the local test:
```
make local-test
```

If you want to investigate the cluster you can do it by:
```
kubectl cluster-info --context kind-source-cluster --kubeconfig test/kubeconfig
kubectl cluster-info --context kind-destination-cluster --kubeconfig test/kubeconfig
```

Once we are done with the test and don't need the local clusters anymore you can delete them with
```
make delete-local-clusters
```
