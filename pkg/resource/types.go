package resource

type Resource struct {
	Kind      string
	Name      string
	Namespace string
}

type FilterConfig struct {
	TrackKinds     []string
	SkipKinds      []string
	TrackResources []Resource
}
