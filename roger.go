package main

import (
	"github.com/56quarters/roger/pkg/app"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
	"net/http"
	"os"
	"time"
)

func setupLogger() *log.Logger {
	logger := log.New()
	logger.SetReportCaller(true)
	logger.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	return logger
}

func main() {
	logger := setupLogger()

	kp := kingpin.New(os.Args[0], "Roger: DNS and networking Prometheus exporter")
	dnsServer := kp.Flag("dns.server", "DNS server to export metrics for, including port").Default("127.0.0.1:53").String()
	dnsPeriod := kp.Flag("dns.period", "How often to poll the DNS server for metrics, as a duration").Default("10s").Duration()

	_, err := kp.Parse(os.Args[1:])
	if err != nil {
		logger.Fatal(err)
	}

	dnsReaderTick := time.NewTicker(*dnsPeriod)
	metricBundle := app.NewMetricBundle(prometheus.DefaultRegisterer)
	dnsmasqReader := app.NewDnsmasqReader(new(dns.Client), *dnsServer)
	defer dnsReaderTick.Stop()

	go func() {
		for {
			select {
			case <-dnsReaderTick.C:
				go func() {
					err := dnsmasqReader.Update(&metricBundle)
					if err != nil {
						logger.Warn(err)
					}
				}()
			}
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	logger.Error(http.ListenAndServe(":8080", nil))
}
