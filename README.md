# Keess

Keess keeps **Secrets** and **ConfigMaps** synchronized across namespaces and remote clusters.

## Usage

Keess uses labels and annotations from Secrets and ConfigMaps to work.

To enable the synchronization you have two steps:

First, you have to add a label indicating which type of synchronization you want: namespace or cluster. See the options below:

`keess.powerhrg.com/sync: namespace`

`keess.powerhrg.com/sync: cluster`

Then you need to configure the synchronization using annotations, which will be described in the next topics.

### Namespace synchronization

The namespace synchronization counts with three options. You can specify the destination namespaces names, and labels or you can choose to synchronize with all namespaces.

`keess.powerhrg.com/namespaces-names: all`

`keess.powerhrg.com/namespaces-names: namespacea, namespaceb, namespacec`

`keess.powerhrg.com/namespace-label: keess.powerhrg.com/sync="true"`


### Cluster synchronization

The cluster synchronization occurs across different clusters and the same namespaces.
You need to specify the cluster names using the annotation below:

`keess.powerhrg.com/clusters: clustera, clusterb, clusterc`
