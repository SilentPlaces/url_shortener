package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/arminaray/url_shortener/services/redirector-service/internal/application"
)

type PromMetrics struct {
	cacheHit  prometheus.Counter
	cacheMiss prometheus.Counter
}

func NewPromMetrics(namespace string) *PromMetrics {
	return &PromMetrics{
		cacheHit: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "cache", Name: "hits_total",
			Help: "Redirector cache hits served from Redis.",
		}),
		cacheMiss: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "cache", Name: "misses_total",
			Help: "Redirector cache misses that fell through to Postgres.",
		}),
	}
}

func (p *PromMetrics) CacheHit(string)  { p.cacheHit.Inc() }
func (p *PromMetrics) CacheMiss(string) { p.cacheMiss.Inc() }

var _ application.Metrics = (*PromMetrics)(nil)
