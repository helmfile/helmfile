package config

// CacheImpl is impl for CacheOptions
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
