package main

import (
	"net/http"
	"os"
	"time"

	"github.com/56quarters/roger/pkg/app"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	kp := kingpin.New(os.Args[0], "Roger: DNS and networking Prometheus exporter")
	metricsPath := kp.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	webAddr := kp.Flag("web.listen-address", "Address and port to expose Prometheus metrics on").Default(":9779").String()
	dnsServer := kp.Flag("dns.server", "DNS server to export metrics for, including port").Default("127.0.0.1:53").String()
	dnsPeriod := kp.Flag("dns.period", "How often to poll the DNS server for metrics, as a duration").Default("10s").Duration()
	procPath := kp.Flag("proc.path", "Path to the proc file system to scrape metrics from").Default("/proc").String()
	procPeriod := kp.Flag("proc.period", "How often to poll the proc file system for metrics, as a duration").Default("10s").Duration()

	_, err := kp.Parse(os.Args[1:])
	if err != nil {
		app.Log.Fatal(err)
	}

	registry := prometheus.DefaultRegisterer

	dnsmasqReader := app.NewDnsmasqReader(new(dns.Client), *dnsServer, app.NewDnsMetrics(registry))
	dnsmasqTick := time.NewTicker(*dnsPeriod)
	defer dnsmasqTick.Stop()

	procReader := app.NewProcReader(*procPath)
	registry.MustRegister(&procReader)
	procTick := time.NewTicker(*procPeriod)
	defer procTick.Stop()

	go func() {
		for {
			select {
			case <-dnsmasqTick.C:
				go func() {
					err := dnsmasqReader.Update()
					if err != nil {
						app.Log.Warn(err)
					}
				}()
			case <-procTick.C:
				go func() {
					err := procReader.Update()
					if err != nil {
						app.Log.Warn(err)
					}
				}()
			}
		}
	}()

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html>
			<head><title>Roger Exporter</title></head>
			<body>
			<h1>Roger Exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			</body>
			</html>`,
		))
	})
	app.Log.Error(http.ListenAndServe(*webAddr, nil))
}
