package abstractions

// Accepted synchronization types.
type SyncType string

const (
	Namespace SyncType = "namespace"
	Cluster   SyncType = "cluster"
)

// Returns the SyncType.
func GetSyncType(syncType string) SyncType {
	if syncType == "namespace" {
		return Namespace
	}

	if syncType == "cluster" {
		return Cluster
	}

	return ""
}
