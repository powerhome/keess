package abstractions

import (
	"k8s.io/apimachinery/pkg/runtime"
)

type EntityEvent struct {
	Type EventType

	Entity runtime.Object
}
