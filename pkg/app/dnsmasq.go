package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
)

type descriptions struct {
	DnsCacheSize       *prometheus.Desc
	DnsCacheInsertions *prometheus.Desc
	DnsCacheEvictions  *prometheus.Desc
	DnsCacheMisses     *prometheus.Desc
	DnsCacheHits       *prometheus.Desc
	DnsAuthoritative   *prometheus.Desc
	DnsUpstreamQueries *prometheus.Desc
	DnsUpstreamErrors  *prometheus.Desc
}

func newDescriptions() *descriptions {
	return &descriptions{
		DnsCacheSize: prometheus.NewDesc(
			"roger_dns_cache_size",
			"Size of the DNS cache",
			[]string{"server"},
			nil,
		),
		DnsCacheInsertions: prometheus.NewDesc(
			"roger_dns_cache_insertions",
			"Number of inserts in the DNS cache",
			[]string{"server"},
			nil,
		),
		DnsCacheEvictions: prometheus.NewDesc(
			"roger_dns_cache_evictions",
			"Number of evictions in the DNS cache",
			[]string{"server"},
			nil,
		),
		DnsCacheMisses: prometheus.NewDesc(
			"roger_dns_cache_misses",
			"Number of misses in the DNS cache",
			[]string{"server"},
			nil,
		),
		DnsCacheHits: prometheus.NewDesc(
			"roger_dns_cache_hits",
			"Number of hits in the DNS cache",
			[]string{"server"},
			nil,
		),
		DnsAuthoritative: prometheus.NewDesc(
			"roger_dns_authoritative",
			"Number of authoritative DNS queries answered",
			[]string{"server"},
			nil,
		),
		DnsUpstreamQueries: prometheus.NewDesc(
			"roger_dns_upstream_queries",
			"Number of queries sent to upstream servers",
			[]string{"server", "upstream"},
			nil,
		),
		DnsUpstreamErrors: prometheus.NewDesc(
			"roger_dns_upstream_errors",
			"Number of errors from upstream servers",
			[]string{"server", "upstream"},
			nil,
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
	client       *dns.Client
	address      string
	descriptions *descriptions
}

func NewDnsmasqReader(client *dns.Client, address string) DnsmasqReader {
	return DnsmasqReader{
		client:       client,
		address:      address,
		descriptions: newDescriptions(),
	}
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

func (d *DnsmasqReader) Describe(ch chan<- *prometheus.Desc) {
	ch <- d.descriptions.DnsCacheSize
	ch <- d.descriptions.DnsCacheInsertions
	ch <- d.descriptions.DnsCacheEvictions
	ch <- d.descriptions.DnsCacheMisses
	ch <- d.descriptions.DnsCacheHits
	ch <- d.descriptions.DnsAuthoritative
	ch <- d.descriptions.DnsUpstreamQueries
	ch <- d.descriptions.DnsUpstreamErrors
}

func (d *DnsmasqReader) Collect(ch chan<- prometheus.Metric) {
	res, err := d.ReadMetrics()
	if err != nil {
		Log.Warnf("Failed to read metrics during collection: %s", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(d.descriptions.DnsCacheSize, prometheus.CounterValue, float64(res.CacheSize), d.address)
	ch <- prometheus.MustNewConstMetric(d.descriptions.DnsCacheInsertions, prometheus.CounterValue, float64(res.CacheInsertions), d.address)
	ch <- prometheus.MustNewConstMetric(d.descriptions.DnsCacheEvictions, prometheus.CounterValue, float64(res.CacheEvictions), d.address)
	ch <- prometheus.MustNewConstMetric(d.descriptions.DnsCacheMisses, prometheus.CounterValue, float64(res.CacheMisses), d.address)
	ch <- prometheus.MustNewConstMetric(d.descriptions.DnsAuthoritative, prometheus.CounterValue, float64(res.Authoritative), d.address)

	for _, s := range res.Servers {
		ch <- prometheus.MustNewConstMetric(d.descriptions.DnsUpstreamQueries, prometheus.CounterValue, float64(s.QueriesSent), d.address, s.Address)
		ch <- prometheus.MustNewConstMetric(d.descriptions.DnsUpstreamErrors, prometheus.CounterValue, float64(s.QueryErrors), d.address, s.Address)
	}
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
