package config

// CacheOptions is the options for the build command
type CacheOptions struct{}

// NewCacheOptions creates a new Apply
func NewCacheOptions() *CacheOptions {
	return &CacheOptions{}
}
