# Metrics

Keess exposes Prometheus metrics to help you monitor the health and performance of the operator. Metrics are available on the `/metrics` endpoint.

Note that Keess creates a new Prometheus metrics registry and does NOT expose the common go metrics automatically provided by Prometheus Go SDK. This is a design choice.

## Available Metrics

### Error Tracking

#### `keess_errors_total`

**Type:** Counter

**Description:** Total number of errors encountered by the operator.

This metric increments whenever Keess encounters an error during its operation. A steady increase in this metric may indicate configuration issues or connectivity problems with clusters.

---

### Resource Management

#### `keess_resources_managed_total`

**Type:** Gauge

**Labels:** `resource_type` (service, configmap, secret, namespace)

**Description:** Total number of resources currently managed by the operator.

These are destination resources being synced FROM other namespaces/clusters. They are created by Keess and have the label `keess.powerhrg.com/managed=true`.

 Resources are counted as "managed" when they match the managed label selector and are being synchronized by Keess.

This is an informational metric to help you understand the scale at which the operator is being used and quickly identify which types of resources are being managed.

**Example:**

```prometheus
keess_resources_managed_total{resource_type="service"} 15
keess_resources_managed_total{resource_type="configmap"} 8
keess_resources_managed_total{resource_type="secret"} 12
```

#### `keess_resources_sync_total`

**Type:** Gauge

**Labels:** `resource_type` (service, configmap, secret, namespace)

**Description:** Total number of resources being synced by the operator.

These are origin resources being synced TO other namespaces/clusters. They are usually created by an user, which sets the `keess.powerhrg.com/sync` label to some supported value. They are the source of synchronization operations.

This is an informational metric to help you understand the scale at which the operator is being used and quickly identify which types of resources are being synced.

**Example:**

```prometheus
keess_resources_sync_total{resource_type="service"} 3
keess_resources_sync_total{resource_type="configmap"} 2
keess_resources_sync_total{resource_type="secret"} 4
```

---

### Orphan Detection and Cleanup

#### `keess_resources_orphan_detections_total`

**Type:** Counter

**Labels:** `resource_type` (service, configmap, secret)

**Description:** Total number of orphaned resources detected by the operator.

This metric increments each time an orphaned resource is detected. An orphan is a managed resource whose source no longer exists or no longer has the sync label.

**Important:** If an orphan cannot be deleted for some reason, it will be counted again the next time it is detected, causing this counter to grow indefinitely while the orphan exists. A steady increase in this metric, or a divergence between `keess_resources_orphan_detections_total` and `keess_resources_orphan_removals_total`, can indicate orphans that may need manual intervention.

#### `keess_resources_orphan_removals_total`

**Type:** Counter

**Labels:** `resource_type` (service, configmap, secret)

**Description:** Total number of orphaned resources successfully removed by the operator.

This metric increments only when an orphan is actually deleted. Compare this with `keess_resources_orphan_detections_total` to identify orphans that cannot be automatically cleaned up.

**Example:**

```prometheus
keess_resources_orphan_detections_total{resource_type="service"} 5
keess_resources_orphan_removals_total{resource_type="service"} 5
```

A divergence between these two metrics indicates orphans that couldn't be removed.

---

### Remote Cluster Connectivity

#### `keess_remote_initialized_success`

**Type:** Gauge

**Labels:** `remote_name` (cluster name)

**Description:** Indicates if the remote cluster was initialized successfully.

- `1` = Remote cluster is accessible and initialized successfully
- `0` = Remote cluster is inaccessible or initialization failed

The remote cluster initialization happens at startup time, and whenever the remote cluster secrets are updated and reloaded. Those events will update this metric. Note however that there is no periodic health check process that will update the metric.

This metric is labeled by remote cluster name, allowing you to track the status of multiple remote clusters independently.

**Example:**

```prometheus
keess_remote_initialized_success{remote_name="cluster1"} 1
keess_remote_initialized_success{remote_name="cluster2"} 0
```

---

### Goroutine Tracking

#### `keess_goroutines`

**Type:** Gauge

**Labels:** `resource_type` (configmap, secret, service, namespace, kubeconfig)

**Description:** Number of active Keess goroutines by resource type.

This metric tracks the number of active goroutines created by Keess for polling, syncing, and deleting resources, as well as watching the kubeconfig file. Those Go routines are created on process startup and should be always up. This metric can help identify problems with those Go routines being finished when they shouldn't.

This is not a complete count of Go routines under the Keess proccess, which will be a more dynamic number.

**Example:**

```prometheus
keess_goroutines{resource_type="configmap"} 2
keess_goroutines{resource_type="secret"} 2
keess_goroutines{resource_type="service"} 2
keess_goroutines{resource_type="namespace"} 1
keess_goroutines{resource_type="kubeconfig"} 1
```
