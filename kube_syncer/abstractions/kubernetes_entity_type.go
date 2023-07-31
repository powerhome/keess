package abstractions

// Accepted kuberentes entity types
type KubernetesEntityType string

const (
	ConfigMapEntity KubernetesEntityType = "configmap"
	SecretEntity    KubernetesEntityType = "secret"
)
