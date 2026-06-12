package application

type Metrics interface {
	CacheHit(alias string)
	CacheMiss(alias string)
	BloomCollision()
	AliasCollision()
	URLCreated(isCustom bool)
}

type NoopMetrics struct{}

func (NoopMetrics) CacheHit(string)  {}
func (NoopMetrics) CacheMiss(string) {}
func (NoopMetrics) BloomCollision()  {}
func (NoopMetrics) AliasCollision()  {}
func (NoopMetrics) URLCreated(bool)  {}
