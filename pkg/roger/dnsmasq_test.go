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

func txt(msgs ...string) dns.RR {
	out := dns.TXT{}
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

	t.Run("mismatched number of questions", func(t *testing.T) {
		// TODO(56quarters) figure out how to test this
		t.Skip()

		var mock mockDNSClient
		mock.msg = &dns.Msg{
			Answer: []dns.RR{
				txt("1000"),
				txt("1001"),
				txt("1002"),
				txt("1003"),
				txt("1004"),
				txt("1005"),
				txt("1006"),
			},
		}

		reader := NewDnsmasqReader(&mock, "127.0.0.1:53", log.NewNopLogger())
		_, err := reader.ReadMetrics()

		assert.ErrorIs(t, err, ErrNumQuestions)
	})

	t.Run("mismatched number of answers", func(t *testing.T) {
		var mock mockDNSClient
		mock.msg = &dns.Msg{
			Answer: []dns.RR{
				txt("1000"),
			},
		}

		reader := NewDnsmasqReader(&mock, "127.0.0.1:53", log.NewNopLogger())
		_, err := reader.ReadMetrics()

		assert.ErrorIs(t, err, ErrNumAnswers)
	})

	t.Run("bad cache size", func(t *testing.T) {
		var mock mockDNSClient
		mock.msg = &dns.Msg{
			Answer: []dns.RR{
				txt("fail"),
				txt("1001"),
				txt("1002"),
				txt("1003"),
				txt("1004"),
				txt("1005"),
				txt("1.1.1.1 1000 500"),
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
				txt("1000"),
				txt("fail"),
				txt("1002"),
				txt("1003"),
				txt("1004"),
				txt("1005"),
				txt("1.1.1.1 1000 500"),
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
				txt("1000"),
				txt("1001"),
				txt("fail"),
				txt("1003"),
				txt("1004"),
				txt("1005"),
				txt("1.1.1.1 1000 500"),
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
				txt("1000"),
				txt("1001"),
				txt("1002"),
				txt("fail"),
				txt("1004"),
				txt("1005"),
				txt("1.1.1.1 1000 500"),
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
				txt("1000"),
				txt("1001"),
				txt("1002"),
				txt("1003"),
				txt("fail"),
				txt("1005"),
				txt("1.1.1.1 1000 500"),
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
				txt("1000"),
				txt("1001"),
				txt("1002"),
				txt("1003"),
				txt("1004"),
				txt("fail"),
				txt("1.1.1.1 1000 500"),
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
				txt("1000"),
				txt("1001"),
				txt("1002"),
				txt("1003"),
				txt("1004"),
				txt("1005"),
				txt("fail"),
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
				txt("1000"),
				txt("1001"),
				txt("1002"),
				txt("1003"),
				txt("1004"),
				txt("1005"),
				txt("1.1.1.1:53 1000 500", "8.8.8.8:53 1001 501"),
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
