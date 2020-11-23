package main

import (
	"github.com/56quarters/roger/pkg/app"
	"github.com/go-kit/kit/log"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"os"
	"time"
)

const (
	dnsReaderFreq = 5 * time.Second
)

func setupLogger() log.Logger {
	var logger log.Logger
	logger = log.NewLogfmtLogger(os.Stderr)
	logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)
	return logger
}

func main() {
	logger := setupLogger()
	dnsServer := os.Args[1]
	dnsReaderTick := time.NewTicker(dnsReaderFreq)
	metricBundle := app.NewMetricBundle(prometheus.DefaultRegisterer)
	dnsmasqReader := app.NewDnsmasqReader(new(dns.Client), dnsServer)
	defer dnsReaderTick.Stop()

	go func() {
		for {
			select {
			case <-dnsReaderTick.C:
				go func() {
					err := dnsmasqReader.Update(&metricBundle)
					if err != nil {
						_ = logger.Log("error", err)
					}
				}()
			}
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	_ = logger.Log("error", http.ListenAndServe(":8080", nil))
}
