
# Keess: Kubernetes Secrets, ConfigMaps, and Services Synchronization

Keess (Keep Stuff Synchronized) is a versatile command-line tool designed to synchronize secrets, configmaps, and services across different namespaces and Kubernetes clusters. Built with simplicity and efficiency in mind, it ensures that your Kubernetes environments are consistently updated, secure, and easy to manage.

## Features

- **Cross-Namespace Synchronization**: Effortlessly sync secrets, configmaps, and services across multiple namespaces within a single Kubernetes cluster.
- **Inter-Cluster Synchronization**: Extend your synchronization capabilities to multiple clusters, keeping your configurations consistent across different environments.
- **Service Synchronization**: Sync services across clusters using Cilium Global Services, enabling seamless cross-cluster service access.
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

#### Service Synchronization

Keess supports synchronizing services across clusters using Cilium Global Services. This feature enables applications in one cluster to access services in another cluster as if they were local.

**Prerequisites:**
- Cilium CNI with ClusterMesh enabled on all participating clusters
- Services must have the `service.cilium.io/global: "true"` annotation

**Configuration:**
1. Add the sync label to your service: `keess.powerhrg.com/sync: cluster`
2. Add the clusters annotation: `keess.powerhrg.com/clusters: clustera, clusterb`
3. Ensure the service has the Cilium global annotation: `service.cilium.io/global: "true"`

**Example:**
```yaml
apiVersion: v1
kind: Service
metadata:
  name: mysql-svc
  namespace: my-namespace
  labels:
    keess.powerhrg.com/sync: "cluster"
  annotations:
    service.cilium.io/global: "true"
    keess.powerhrg.com/clusters: "cluster-b, cluster-c"
spec:
  ports:
  - name: mysql
    port: 3306
    protocol: TCP
    targetPort: 3306
  selector:
    app.kubernetes.io/component: mysql
  type: ClusterIP
```

Keess will automatically create service references in the target clusters with:
- Same name and namespace as the source service
- Cilium annotations for global service configuration
- Empty selector (no local endpoints)
- Keess management labels and annotations

**Note:** Service synchronization only supports cluster-level sync. Namespace-level sync for services is not supported.

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
