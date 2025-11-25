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

#### `keess_remote_initialization_failed`

**Type:** Gauge

**Labels:** `remote_name` (cluster name)

**Description:** Indicates if the remote cluster initialization has failed.

- `0` = Remote cluster is accessible and initialized successfully
- `1` = Remote cluster is inaccessible or initialization failed

The remote cluster initialization happens at startup time, and whenever the remote
cluster secrets are updated and reloaded. Those events will update this metric. Note
however that there is no periodic health check process that will update the metric.

This metric is labeled by remote cluster name, allowing you to track the status of
multiple remote clusters independently.

**Example:**

```prometheus
keess_remote_initialization_failed{remote_name="cluster1"} 1
keess_remote_initialization_failed{remote_name="cluster2"} 0
```

---

### Goroutine Tracking

#### `keess_goroutines_inactive`

**Type:** Gauge

**Labels:** `resource_type` (configmap, secret, service, namespace, kubeconfig)

**Description:** Number of inactive Keess goroutines by resource type.

This metric tracks the number of inactive goroutines, but only for the main goroutines
created by Keess to poll, sync, and delete resources, and watch the kubeconfig file.

This metric can help identify problems with those Go routines being finished when they
shouldn't. The expected count of those goroutines is static and known to Keess, so any
number > 0 here indicates a problem, if sync is enabled for that resource type.

**Example:**

```prometheus
keess_goroutines_inactive{resource_type="configmap"} 0
keess_goroutines_inactive{resource_type="secret"} 0
keess_goroutines_inactive{resource_type="service"} 0
keess_goroutines_inactive{resource_type="namespace"} 0
keess_goroutines_inactive{resource_type="kubeconfig"} 0
```

---

## Using the Metrics

### Prometheus Discovery

Keess supports two methods for Prometheus to discover and scrape metrics:

#### Method 1: ServiceMonitor (Recommended for Prometheus Operator)

If you're using the Prometheus Operator, enable the ServiceMonitor in your Helm values:

```yaml
metrics:
  serviceMonitor:
    enabled: true
    interval: 30s
    scrapeTimeout: 10s
```

The ServiceMonitor resource will be automatically created and Prometheus will discover the keess metrics endpoint.

#### Method 2: Annotation-based Discovery

The keess Service includes Prometheus scrape annotations by default:

```yaml
prometheus.io/scrape: "true"
prometheus.io/port: "8080"
prometheus.io/path: "/metrics"
```

If your Prometheus is configured to discover services based on annotations, it will automatically find and scrape keess. Ensure your Prometheus configuration includes a job that discovers services with these annotations:

```yaml
scrape_configs:
  - job_name: 'kubernetes-service-endpoints'
    kubernetes_sd_configs:
      - role: endpoints
    relabel_configs:
      - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scrape]
        action: keep
        regex: true
      - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_path]
        action: replace
        target_label: __metrics_path__
        regex: (.+)
      - source_labels: [__address__, __meta_kubernetes_service_annotation_prometheus_io_port]
        action: replace
        regex: ([^:]+)(?::\d+)?;(\d+)
        replacement: $1:$2
        target_label: __address__
```

#### Method 3: Static Configuration

If using a custom configuration (not ServiceMonitor), you can add Keess manually:

```yaml
scrape_configs:
  - job_name: 'keess'
    static_configs:
      - targets: ['keess-service:8080']
```
