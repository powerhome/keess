
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

Since it depends on Cilium, it's disabled by default. You need to pass `--enableServiceSync=true` to enable it

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

## Debugging and Profiling

You can turn on debug level log message by setting `--logLevel debug`.

Also, Keess includes optional runtime profiling support via Go's pprof package for performance analysis and debugging. To enable the pprof server, use the `--enablePprof` flag.

When using the Helm chart, enable those by setting:

```yaml
logLevel: debug
enablePprof: true
```

**Security Note:** Only enable pprof in development or controlled environments, as it exposes runtime information that could be sensitive.

### Using Profiling

When enabled, the pprof server starts on port 6060 and provides the following endpoints:

- `http://localhost:6060/debug/pprof/` - Main pprof index
- `http://localhost:6060/debug/pprof/goroutine` - Goroutine analysis
- `http://localhost:6060/debug/pprof/profile` - CPU profiling
- `http://localhost:6060/debug/pprof/heap` - Memory profiling

Example commands:

```shell
# Analyze current goroutines
go tool pprof http://localhost:6060/debug/pprof/goroutine

# Capture 30-second CPU profile
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# View memory allocation
go tool pprof http://localhost:6060/debug/pprof/heap

# Quick goroutine dump
curl http://localhost:6060/debug/pprof/goroutine?debug=1
```

## Contributing

Contributions are welcome! Please refer to our [Contributing Guidelines](CONTRIBUTING.md) for more information.

## Support

If you encounter any issues or have questions, please file an issue on the [GitHub Issues page](https://github.com/your-repo/keess/issues).

## License

Keess is open-source software licensed under the MIT license. See the [LICENSE](LICENSE) file for details.

## Local testing

See [tests/README.md](tests/README.md)
