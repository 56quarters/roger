package app

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type MetricBundle struct {
	DnsCacheSize       prometheus.Gauge
	DnsCacheInsertions prometheus.Gauge
	DnsCacheEvictions  prometheus.Gauge
	DnsCacheMisses     prometheus.Gauge
	DnsCacheHits       prometheus.Gauge
	DnsAuthoritative   prometheus.Gauge
	DnsUpstreamQueries *prometheus.GaugeVec
	DnsUpstreamErrors  *prometheus.GaugeVec
}

func NewMetricBundle(r prometheus.Registerer) MetricBundle {
	return MetricBundle{
		DnsCacheSize: promauto.With(r).NewGauge(prometheus.GaugeOpts{
			Name: "roger_dns_cache_size",
			Help: "Size of the DNS cache",
		}),
		DnsCacheInsertions: promauto.With(r).NewGauge(prometheus.GaugeOpts{
			Name: "roger_dns_cache_insertions",
			Help: "Number of inserts in the DNS cache",
		}),
		DnsCacheEvictions: promauto.With(r).NewGauge(prometheus.GaugeOpts{
			Name: "roger_dns_cache_evictions",
			Help: "Number of evictions in the DNS cache",
		}),
		DnsCacheMisses: promauto.With(r).NewGauge(prometheus.GaugeOpts{
			Name: "roger_dns_cache_misses",
			Help: "Number of misses in the DNS cache",
		}),
		DnsCacheHits: promauto.With(r).NewGauge(prometheus.GaugeOpts{
			Name: "roger_dns_cache_hits",
			Help: "Number of hits in the DNS cache",
		}),
		DnsAuthoritative: promauto.With(r).NewGauge(prometheus.GaugeOpts{
			Name: "roger_dns_authoritative",
			Help: "Number of authoritative DNS queries answered",
		}),
		DnsUpstreamQueries: promauto.With(r).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "roger_dns_upstream_queries",
				Help: "Number of queries sent to upstream servers",
			},
			[]string{"server"},
		),
		DnsUpstreamErrors: promauto.With(r).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "roger_dns_upstream_errors",
				Help: "Number of errors from upstream servers",
			},
			[]string{"server"},
		),
	}
}

type Recorder interface {
	Update(bundle *MetricBundle) error
}
