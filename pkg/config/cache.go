package config

// CacheOptions is the options for the build command
type CacheOptions struct{}

// NewCacheOptions creates a new Apply
func NewCacheOptions() *CacheOptions {
	return &CacheOptions{}
}

// CacheImpl is impl for applyOptions
type CacheImpl struct {
	*GlobalImpl
	*CacheOptions
}

// NewCacheImpl creates a new CacheImpl
func NewCacheImpl(g *GlobalImpl, b *CacheOptions) *CacheImpl {
	return &CacheImpl{
		GlobalImpl:   g,
		CacheOptions: b,
	}
}

// Args returns the args.
func (b *CacheImpl) Args() string {
	return ""
}
