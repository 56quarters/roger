package roger

import (
	"errors"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDNSClient struct {
	err error
	msg *dns.Msg
}

func (c *mockDNSClient) Exchange(q *dns.Msg, _ string) (r *dns.Msg, rtt time.Duration, err error) {
	if c.err != nil {
		return nil, 0, c.err
	}

	var msg dns.Msg
	msg.Question = q.Question
	msg.Answer = c.msg.Answer

	return &msg, 1 * time.Second, nil
}

func txt(name string, msgs ...string) dns.RR {
	out := dns.TXT{}
	out.Hdr = dns.RR_Header{Name: name}
	out.Txt = msgs
	return &out
}

func TestDnsmasqReader_ReadMetrics(t *testing.T) {
	t.Run("client exchange error", func(t *testing.T) {
		var mock mockDNSClient
		mock.err = errors.New("dns client error")

		reader := NewDnsmasqReader(&mock, "127.0.0.1:53", log.NewNopLogger())
		_, err := reader.ReadMetrics()

		assert.ErrorIs(t, err, ErrUpstream)
	})

	t.Run("bad cache size", func(t *testing.T) {
		var mock mockDNSClient
		mock.msg = &dns.Msg{
			Answer: []dns.RR{
				txt("cachesize.bind.", "fail"),
				txt("insertions.bind.", "1001"),
				txt("evictions.bind.", "1002"),
				txt("misses.bind.", "1003"),
				txt("hits.bind.", "1004"),
				txt("auth.bind.", "1005"),
				txt("servers.bind.", "1.1.1.1 1000 500"),
			},
		}

		reader := NewDnsmasqReader(&mock, "127.0.0.1:53", log.NewNopLogger())
		_, err := reader.ReadMetrics()

		assert.ErrorIs(t, err, ErrParseAnswer)
	})

	t.Run("bad cache insertions", func(t *testing.T) {
		var mock mockDNSClient
		mock.msg = &dns.Msg{
			Answer: []dns.RR{
				txt("cachesize.bind.", "1000"),
				txt("insertions.bind.", "fail"),
				txt("evictions.bind.", "1002"),
				txt("misses.bind.", "1003"),
				txt("hits.bind.", "1004"),
				txt("auth.bind.", "1005"),
				txt("servers.bind.", "1.1.1.1 1000 500"),
			},
		}

		reader := NewDnsmasqReader(&mock, "127.0.0.1:53", log.NewNopLogger())
		_, err := reader.ReadMetrics()

		assert.ErrorIs(t, err, ErrParseAnswer)
	})

	t.Run("bad cache evictions", func(t *testing.T) {
		var mock mockDNSClient
		mock.msg = &dns.Msg{
			Answer: []dns.RR{
				txt("cachesize.bind.", "1000"),
				txt("insertions.bind.", "1001"),
				txt("evictions.bind.", "fail"),
				txt("misses.bind.", "1003"),
				txt("hits.bind.", "1004"),
				txt("auth.bind.", "1005"),
				txt("servers.bind.", "1.1.1.1 1000 500"),
			},
		}

		reader := NewDnsmasqReader(&mock, "127.0.0.1:53", log.NewNopLogger())
		_, err := reader.ReadMetrics()

		assert.ErrorIs(t, err, ErrParseAnswer)
	})

	t.Run("bad cache misses", func(t *testing.T) {
		var mock mockDNSClient
		mock.msg = &dns.Msg{
			Answer: []dns.RR{
				txt("cachesize.bind.", "1000"),
				txt("insertions.bind.", "1001"),
				txt("evictions.bind.", "1002"),
				txt("misses.bind.", "fail"),
				txt("hits.bind.", "1004"),
				txt("auth.bind.", "1005"),
				txt("servers.bind.", "1.1.1.1 1000 500"),
			},
		}

		reader := NewDnsmasqReader(&mock, "127.0.0.1:53", log.NewNopLogger())
		_, err := reader.ReadMetrics()

		assert.ErrorIs(t, err, ErrParseAnswer)
	})

	t.Run("bad cache hits", func(t *testing.T) {
		var mock mockDNSClient
		mock.msg = &dns.Msg{
			Answer: []dns.RR{
				txt("cachesize.bind.", "1000"),
				txt("insertions.bind.", "1001"),
				txt("evictions.bind.", "1002"),
				txt("misses.bind.", "1003"),
				txt("hits.bind.", "fail"),
				txt("auth.bind.", "1005"),
				txt("servers.bind.", "1.1.1.1 1000 500"),
			},
		}

		reader := NewDnsmasqReader(&mock, "127.0.0.1:53", log.NewNopLogger())
		_, err := reader.ReadMetrics()

		assert.ErrorIs(t, err, ErrParseAnswer)
	})

	t.Run("bad authoritative", func(t *testing.T) {
		var mock mockDNSClient
		mock.msg = &dns.Msg{
			Answer: []dns.RR{
				txt("cachesize.bind.", "1000"),
				txt("insertions.bind.", "1001"),
				txt("evictions.bind.", "1002"),
				txt("misses.bind.", "1003"),
				txt("hits.bind.", "1004"),
				txt("auth.bind.", "fail"),
				txt("servers.bind.", "1.1.1.1 1000 500"),
			},
		}

		reader := NewDnsmasqReader(&mock, "127.0.0.1:53", log.NewNopLogger())
		_, err := reader.ReadMetrics()

		assert.ErrorIs(t, err, ErrParseAnswer)
	})

	t.Run("bad servers", func(t *testing.T) {
		var mock mockDNSClient
		mock.msg = &dns.Msg{
			Answer: []dns.RR{
				txt("cachesize.bind.", "1000"),
				txt("insertions.bind.", "1001"),
				txt("evictions.bind.", "1002"),
				txt("misses.bind.", "1003"),
				txt("hits.bind.", "1004"),
				txt("auth.bind.", "1005"),
				txt("servers.bind.", "fail"),
			},
		}

		reader := NewDnsmasqReader(&mock, "127.0.0.1:53", log.NewNopLogger())
		_, err := reader.ReadMetrics()

		assert.ErrorIs(t, err, ErrParseAnswer)
	})

	t.Run("success", func(t *testing.T) {
		var mock mockDNSClient
		mock.msg = &dns.Msg{
			Answer: []dns.RR{
				txt("cachesize.bind.", "1000"),
				txt("insertions.bind.", "1001"),
				txt("evictions.bind.", "1002"),
				txt("misses.bind.", "1003"),
				txt("hits.bind.", "1004"),
				txt("auth.bind.", "1005"),
				txt("servers.bind.", "1.1.1.1:53 1000 500", "8.8.8.8:53 1001 501"),
			},
		}

		reader := NewDnsmasqReader(&mock, "127.0.0.1:53", log.NewNopLogger())
		res, err := reader.ReadMetrics()

		require.NoError(t, err)
		assert.Equal(t, uint64(1000), res.CacheSize)
		assert.Equal(t, uint64(1001), res.CacheInsertions)
		assert.Equal(t, uint64(1002), res.CacheEvictions)
		assert.Equal(t, uint64(1003), res.CacheMisses)
		assert.Equal(t, uint64(1004), res.CacheHits)
		assert.Equal(t, uint64(1005), res.Authoritative)

		require.Len(t, res.Servers, 2)
		assert.Equal(t, "1.1.1.1:53", res.Servers[0].Address)
		assert.Equal(t, uint64(1000), res.Servers[0].QueriesSent)
		assert.Equal(t, uint64(500), res.Servers[0].QueryErrors)
		assert.Equal(t, "8.8.8.8:53", res.Servers[1].Address)
		assert.Equal(t, uint64(1001), res.Servers[1].QueriesSent)
		assert.Equal(t, uint64(501), res.Servers[1].QueryErrors)
	})
}
