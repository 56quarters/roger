package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type DnsMetrics struct {
	DnsCacheSize       *prometheus.GaugeVec
	DnsCacheInsertions *prometheus.GaugeVec
	DnsCacheEvictions  *prometheus.GaugeVec
	DnsCacheMisses     *prometheus.GaugeVec
	DnsCacheHits       *prometheus.GaugeVec
	DnsAuthoritative   *prometheus.GaugeVec
	DnsUpstreamQueries *prometheus.GaugeVec
	DnsUpstreamErrors  *prometheus.GaugeVec
}

func NewDnsMetrics(r prometheus.Registerer) *DnsMetrics {
	return &DnsMetrics{
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

type DnsmasqResult struct {
	CacheSize       uint64
	CacheInsertions uint64
	CacheEvictions  uint64
	CacheMisses     uint64
	CacheHits       uint64
	Authoritative   uint64
	Servers         []ServerStats
}

type ServerStats struct {
	Address     string
	QueriesSent uint64
	QueryErrors uint64
}

type DnsmasqReader struct {
	client  *dns.Client
	address string
	bundle  *DnsMetrics
}

func NewDnsmasqReader(client *dns.Client, address string, bundle *DnsMetrics) DnsmasqReader {
	return DnsmasqReader{
		client:  client,
		address: address,
		bundle:  bundle,
	}
}

func (d *DnsmasqReader) Update() error {
	res, err := d.ReadMetrics()
	if err != nil {
		return err
	}

	d.bundle.DnsCacheSize.WithLabelValues(d.address).Set(float64(res.CacheSize))
	d.bundle.DnsCacheInsertions.WithLabelValues(d.address).Set(float64(res.CacheInsertions))
	d.bundle.DnsCacheEvictions.WithLabelValues(d.address).Set(float64(res.CacheEvictions))
	d.bundle.DnsCacheMisses.WithLabelValues(d.address).Set(float64(res.CacheMisses))
	d.bundle.DnsCacheHits.WithLabelValues(d.address).Set(float64(res.CacheHits))
	d.bundle.DnsAuthoritative.WithLabelValues(d.address).Set(float64(res.Authoritative))

	for _, s := range res.Servers {
		d.bundle.DnsUpstreamQueries.WithLabelValues(d.address, s.Address).Set(float64(s.QueriesSent))
		d.bundle.DnsUpstreamErrors.WithLabelValues(d.address, s.Address).Set(float64(s.QueryErrors))
	}

	return nil
}

// Make a DNS request to get all known dnsmasq metrics
func (d *DnsmasqReader) ReadMetrics() (*DnsmasqResult, error) {
	m := new(dns.Msg)
	m.MsgHdr = dns.MsgHdr{
		Id:               dns.Id(),
		RecursionDesired: true,
	}
	m.Question = []dns.Question{
		question("cachesize.bind."),
		question("insertions.bind."),
		question("evictions.bind."),
		question("misses.bind."),
		question("hits.bind."),
		question("auth.bind."),
		question("servers.bind."),
	}

	res, _, err := d.client.Exchange(m, d.address)
	if err != nil {
		return nil, err
	}

	if len(res.Answer) != len(res.Question) {
		return nil, fmt.Errorf(
			"unexpected number of answers from %s (%d expected, got %d)",
			d.address, len(m.Question), len(res.Answer),
		)
	}

	cacheSize, err := parseIntRecord(res.Answer[0])
	if err != nil {
		return nil, err
	}

	cacheInsertions, err := parseIntRecord(res.Answer[1])
	if err != nil {
		return nil, err
	}

	cacheEvictions, err := parseIntRecord(res.Answer[2])
	if err != nil {
		return nil, err
	}

	cacheMisses, err := parseIntRecord(res.Answer[3])
	if err != nil {
		return nil, err
	}

	cacheHits, err := parseIntRecord(res.Answer[4])
	if err != nil {
		return nil, err
	}

	authoritative, err := parseIntRecord(res.Answer[5])
	if err != nil {
		return nil, err
	}

	servers, err := parseServersRecord(res.Answer[6])
	if err != nil {
		return nil, err
	}

	return &DnsmasqResult{
		CacheSize:       cacheSize,
		CacheInsertions: cacheInsertions,
		CacheEvictions:  cacheEvictions,
		CacheMisses:     cacheMisses,
		CacheHits:       cacheHits,
		Authoritative:   authoritative,
		Servers:         servers,
	}, nil
}

func parseIntRecord(answer dns.RR) (uint64, error) {
	txt := answer.(*dns.TXT)
	parsed, err := strconv.ParseUint(txt.Txt[0], 10, 64)
	if err != nil {
		return 0, err
	}

	return parsed, nil
}

func parseServersRecord(answer dns.RR) ([]ServerStats, error) {
	txt := answer.(*dns.TXT)
	out := make([]ServerStats, len(txt.Txt))

	for i, val := range txt.Txt {
		statParts := strings.Split(val, " ")
		sent, err := strconv.ParseUint(statParts[1], 10, 64)
		if err != nil {
			return nil, err
		}

		errors, err := strconv.ParseUint(statParts[2], 10, 64)
		if err != nil {
			return nil, err
		}

		out[i] = ServerStats{
			Address:     statParts[0],
			QueriesSent: sent,
			QueryErrors: errors,
		}
	}

	return out, nil
}

func question(name string) dns.Question {
	return dns.Question{Name: name, Qtype: dns.TypeTXT, Qclass: dns.ClassCHAOS}
}
