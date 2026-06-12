package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/application"
)

type PromMetrics struct {
	cacheHit       prometheus.Counter
	cacheMiss      prometheus.Counter
	bloomCollision prometheus.Counter
	aliasCollision prometheus.Counter
	urlCreated     *prometheus.CounterVec
}

func NewPromMetrics(namespace string) *PromMetrics {
	return &PromMetrics{
		cacheHit: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "cache", Name: "hits_total",
			Help: "Total cache hits served from Redis.",
		}),
		cacheMiss: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "cache", Name: "misses_total",
			Help: "Total cache misses that fell through to Postgres.",
		}),
		bloomCollision: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "bloom", Name: "collisions_total",
			Help: "Generated aliases skipped because the bloom filter flagged them.",
		}),
		aliasCollision: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "alias", Name: "collisions_total",
			Help: "Alias inserts rejected by Postgres unique constraint.",
		}),
		urlCreated: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "url", Name: "created_total",
			Help: "Successfully created short URLs, partitioned by alias source.",
		}, []string{"source"}),
	}
}

func (p *PromMetrics) CacheHit(string)  { p.cacheHit.Inc() }
func (p *PromMetrics) CacheMiss(string) { p.cacheMiss.Inc() }
func (p *PromMetrics) BloomCollision()  { p.bloomCollision.Inc() }
func (p *PromMetrics) AliasCollision()  { p.aliasCollision.Inc() }

func (p *PromMetrics) URLCreated(isCustom bool) {
	source := "generated"
	if isCustom {
		source = "custom"
	}
	p.urlCreated.WithLabelValues(source).Inc()
}

var _ application.Metrics = (*PromMetrics)(nil)
