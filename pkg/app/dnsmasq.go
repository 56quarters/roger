package app

import (
	"fmt"
	"github.com/miekg/dns"
	"strconv"
	"strings"
)

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
}

func NewDnsmasqReader(client *dns.Client, address string) DnsmasqReader {
	return DnsmasqReader{
		client:  client,
		address: address,
	}
}

func (d *DnsmasqReader) Update(bundle *MetricBundle) error {
	res, err := d.ReadMetrics()
	if err != nil {
		return err
	}

	bundle.DnsCacheSize.Set(float64(res.CacheSize))
	bundle.DnsCacheInsertions.Set(float64(res.CacheInsertions))
	bundle.DnsCacheEvictions.Set(float64(res.CacheEvictions))
	bundle.DnsCacheMisses.Set(float64(res.CacheMisses))
	bundle.DnsCacheHits.Set(float64(res.CacheHits))
	bundle.DnsAuthoritative.Set(float64(res.Authoritative))

	for _, s := range res.Servers {
		bundle.DnsUpstreamQueries.WithLabelValues(s.Address).Set(float64(s.QueriesSent))
		bundle.DnsUpstreamErrors.WithLabelValues(s.Address).Set(float64(s.QueryErrors))
	}

	return nil
}

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
