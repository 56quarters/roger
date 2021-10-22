// Roger - DNS and network metrics exporter for Prometheus
//
// Copyright 2020-2021 Nick Pillitteri
//
// Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
// http://www.apache.org/licenses/LICENSE-2.0> or the MIT license
// <LICENSE-MIT or http://opensource.org/licenses/MIT>, at your
// option. This file may not be copied, modified, or distributed
// except according to those terms.

package roger

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	ErrUpstream     = errors.New("error calling upstream")
	ErrNumAnswers   = errors.New("unexpected number of answers")
	ErrNumQuestions = errors.New("unexpected number of questions")
	ErrParseAnswer  = errors.New("error parsing answer")
)

// dnsClient is an interface for to allow testing of DnsmasqReader
type dnsClient interface {
	Exchange(m *dns.Msg, address string) (r *dns.Msg, rtt time.Duration, err error)
}

type descriptions struct {
	dnsCacheSize       *prometheus.Desc
	dnsCacheInsertions *prometheus.Desc
	dnsCacheEvictions  *prometheus.Desc
	dnsCacheMisses     *prometheus.Desc
	dnsCacheHits       *prometheus.Desc
	dnsAuthoritative   *prometheus.Desc
	dnsUpstreamQueries *prometheus.Desc
	dnsUpstreamErrors  *prometheus.Desc
}

func newDescriptions() *descriptions {
	return &descriptions{
		dnsCacheSize: prometheus.NewDesc(
			"roger_dns_cache_size",
			"Size of the DNS cache",
			[]string{"server"},
			nil,
		),
		dnsCacheInsertions: prometheus.NewDesc(
			"roger_dns_cache_insertions",
			"Number of inserts in the DNS cache",
			[]string{"server"},
			nil,
		),
		dnsCacheEvictions: prometheus.NewDesc(
			"roger_dns_cache_evictions",
			"Number of evictions in the DNS cache",
			[]string{"server"},
			nil,
		),
		dnsCacheMisses: prometheus.NewDesc(
			"roger_dns_cache_misses",
			"Number of misses in the DNS cache",
			[]string{"server"},
			nil,
		),
		dnsCacheHits: prometheus.NewDesc(
			"roger_dns_cache_hits",
			"Number of hits in the DNS cache",
			[]string{"server"},
			nil,
		),
		dnsAuthoritative: prometheus.NewDesc(
			"roger_dns_authoritative",
			"Number of authoritative DNS queries answered",
			[]string{"server"},
			nil,
		),
		dnsUpstreamQueries: prometheus.NewDesc(
			"roger_dns_upstream_queries",
			"Number of queries sent to upstream servers",
			[]string{"server", "upstream"},
			nil,
		),
		dnsUpstreamErrors: prometheus.NewDesc(
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
	client       dnsClient
	address      string
	descriptions *descriptions
	logger       log.Logger
}

func NewDnsmasqReader(client dnsClient, address string, logger log.Logger) *DnsmasqReader {
	return &DnsmasqReader{
		client:       client,
		address:      address,
		descriptions: newDescriptions(),
		logger:       logger,
	}
}

// ReadMetrics makes a DNS request to get all known dnsmasq metrics
func (d *DnsmasqReader) ReadMetrics() (*DnsmasqResult, error) {
	m := &dns.Msg{}
	m.MsgHdr = dns.MsgHdr{Id: dns.Id(), RecursionDesired: true}
	m.Question = []dns.Question{
		question("cachesize.bind."),
		question("insertions.bind."),
		question("evictions.bind."),
		question("misses.bind."),
		question("hits.bind."),
		question("auth.bind."),
		question("servers.bind."),
	}

	// TODO(56quarters) emit RTT as a metric
	res, _, err := d.client.Exchange(m, d.address)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUpstream, err)
	}

	// Make sure the questions we sent were included in the response
	if len(res.Question) != len(m.Question) {
		return nil, fmt.Errorf(
			"%w from %s (%d expected, %d received)",
			ErrNumQuestions, d.address, len(m.Question), len(res.Question),
		)
	}

	// Make sure the number of answers matches the number of questions
	if len(res.Answer) != len(res.Question) {
		return nil, fmt.Errorf(
			"%w from %s (%d expected, got %d)",
			ErrNumAnswers, d.address, len(m.Question), len(res.Answer),
		)
	}

	cacheSize, err := parseIntRecord(res.Answer[0])
	if err != nil {
		return nil, fmt.Errorf("%w cache size: %s", ErrParseAnswer, err)
	}

	cacheInsertions, err := parseIntRecord(res.Answer[1])
	if err != nil {
		return nil, fmt.Errorf("%w cache insertions: %s", ErrParseAnswer, err)
	}

	cacheEvictions, err := parseIntRecord(res.Answer[2])
	if err != nil {
		return nil, fmt.Errorf("%w cache evictions: %s", ErrParseAnswer, err)
	}

	cacheMisses, err := parseIntRecord(res.Answer[3])
	if err != nil {
		return nil, fmt.Errorf("%w cache misses: %s", ErrParseAnswer, err)
	}

	cacheHits, err := parseIntRecord(res.Answer[4])
	if err != nil {
		return nil, fmt.Errorf("%w cache hits: %s", ErrParseAnswer, err)
	}

	authoritative, err := parseIntRecord(res.Answer[5])
	if err != nil {
		return nil, fmt.Errorf("%w authoritative: %s", ErrParseAnswer, err)
	}

	servers, err := parseServersRecord(res.Answer[6])
	if err != nil {
		return nil, fmt.Errorf("%w servers: %s", ErrParseAnswer, err)
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
	ch <- d.descriptions.dnsCacheSize
	ch <- d.descriptions.dnsCacheInsertions
	ch <- d.descriptions.dnsCacheEvictions
	ch <- d.descriptions.dnsCacheMisses
	ch <- d.descriptions.dnsCacheHits
	ch <- d.descriptions.dnsAuthoritative
	ch <- d.descriptions.dnsUpstreamQueries
	ch <- d.descriptions.dnsUpstreamErrors
}

func (d *DnsmasqReader) Collect(ch chan<- prometheus.Metric) {
	res, err := d.ReadMetrics()
	if err != nil {
		level.Error(d.logger).Log("msg", "failed to read dnsmasq metrics during collection", "addr", d.address, "err", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(d.descriptions.dnsCacheSize, prometheus.CounterValue, float64(res.CacheSize), d.address)
	ch <- prometheus.MustNewConstMetric(d.descriptions.dnsCacheInsertions, prometheus.CounterValue, float64(res.CacheInsertions), d.address)
	ch <- prometheus.MustNewConstMetric(d.descriptions.dnsCacheEvictions, prometheus.CounterValue, float64(res.CacheEvictions), d.address)
	ch <- prometheus.MustNewConstMetric(d.descriptions.dnsCacheMisses, prometheus.CounterValue, float64(res.CacheMisses), d.address)
	ch <- prometheus.MustNewConstMetric(d.descriptions.dnsCacheHits, prometheus.CounterValue, float64(res.CacheHits), d.address)
	ch <- prometheus.MustNewConstMetric(d.descriptions.dnsAuthoritative, prometheus.CounterValue, float64(res.Authoritative), d.address)

	for _, s := range res.Servers {
		ch <- prometheus.MustNewConstMetric(d.descriptions.dnsUpstreamQueries, prometheus.CounterValue, float64(s.QueriesSent), d.address, s.Address)
		ch <- prometheus.MustNewConstMetric(d.descriptions.dnsUpstreamErrors, prometheus.CounterValue, float64(s.QueryErrors), d.address, s.Address)
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
		if len(statParts) != 3 {
			return nil, fmt.Errorf("expected 3 server fields, got %d from %s", len(statParts), val)
		}

		queriesSent, err := strconv.ParseUint(statParts[1], 10, 64)
		if err != nil {
			return nil, err
		}

		queryErrors, err := strconv.ParseUint(statParts[2], 10, 64)
		if err != nil {
			return nil, err
		}

		out[i] = ServerStats{
			Address:     statParts[0],
			QueriesSent: queriesSent,
			QueryErrors: queryErrors,
		}
	}

	return out, nil
}

func question(name string) dns.Question {
	return dns.Question{Name: name, Qtype: dns.TypeTXT, Qclass: dns.ClassCHAOS}
}
