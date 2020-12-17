package app

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type MetricBundle struct {
	DnsCacheSize       *prometheus.GaugeVec
	DnsCacheInsertions *prometheus.GaugeVec
	DnsCacheEvictions  *prometheus.GaugeVec
	DnsCacheMisses     *prometheus.GaugeVec
	DnsCacheHits       *prometheus.GaugeVec
	DnsAuthoritative   *prometheus.GaugeVec
	DnsUpstreamQueries *prometheus.GaugeVec
	DnsUpstreamErrors  *prometheus.GaugeVec
}

func NewMetricBundle(r prometheus.Registerer) MetricBundle {
	return MetricBundle{
		DnsCacheSize: promauto.With(r).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "roger_dns_cache_size",
				Help: "Size of the DNS cache",
			},
			[]string{"server"},
		),
		DnsCacheInsertions: promauto.With(r).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "roger_dns_cache_insertions",
				Help: "Number of inserts in the DNS cache",
			},
			[]string{"server"},
		),
		DnsCacheEvictions: promauto.With(r).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "roger_dns_cache_evictions",
				Help: "Number of evictions in the DNS cache",
			},
			[]string{"server"},
		),
		DnsCacheMisses: promauto.With(r).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "roger_dns_cache_misses",
				Help: "Number of misses in the DNS cache",
			},
			[]string{"server"},
		),
		DnsCacheHits: promauto.With(r).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "roger_dns_cache_hits",
				Help: "Number of hits in the DNS cache",
			},
			[]string{"server"},
		),
		DnsAuthoritative: promauto.With(r).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "roger_dns_authoritative",
				Help: "Number of authoritative DNS queries answered",
			},
			[]string{"server"},
		),
		DnsUpstreamQueries: promauto.With(r).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "roger_dns_upstream_queries",
				Help: "Number of queries sent to upstream servers",
			},
			[]string{"server", "upstream"},
		),
		DnsUpstreamErrors: promauto.With(r).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "roger_dns_upstream_errors",
				Help: "Number of errors from upstream servers",
			},
			[]string{"server", "upstream"},
		),
	}
}

type Recorder interface {
	Update(bundle *MetricBundle) error
}
