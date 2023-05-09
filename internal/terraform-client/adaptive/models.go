package adaptive

// A resource is a adaptive-resource / integration to 3rd party software like databases, server, cluster
// that adaptive delegates access to
type Resource interface {
	// GetID() string
	// GetName() string
}

type Mongo struct {
	ID   string
	Name string
	Uri  string
}
