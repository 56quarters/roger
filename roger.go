// Roger - DNS and network metrics exporter for Prometheus
//
// Copyright 2020-2021 Nick Pillitteri
//
// Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
// http://www.apache.org/licenses/LICENSE-2.0> or the MIT license
// <LICENSE-MIT or http://opensource.org/licenses/MIT>, at your
// option. This file may not be copied, modified, or distributed
// except according to those terms.

package main

import (
	"html/template"
	"net/http"
	"os"
	"runtime"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/56quarters/roger/pkg/roger"
)

// Set by the build process: -ldflags="-X 'main.Version=xyz'"
var (
	Version  string
	Branch   string
	Revision string
)

const indexTpt = `
<!doctype html>
<html>
<head><title>Roger Exporter</title></head>
<body>
<h1>Roger Exporter</h1>
<p><a href="{{ . }}">Metrics</a></p>
</body>
</html>
`

func setupLogger(l level.Option) log.Logger {
	logger := log.NewSyncLogger(log.NewLogfmtLogger(os.Stderr))
	logger = level.NewFilter(logger, l)
	logger = log.With(logger, "ts", log.DefaultTimestampUTC)
	return logger
}

func main() {
	logger := setupLogger(level.AllowInfo())

	kp := kingpin.New(os.Args[0], "Roger: DNS and network metrics exporter for Prometheus")
	metricsPath := kp.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	webAddr := kp.Flag("web.listen-address", "Address and port to expose Prometheus metrics on").Default(":9779").String()
	dnsServer := kp.Flag("dns.server", "DNS server to export metrics for, including port").Default("127.0.0.1:53").String()
	procPath := kp.Flag("proc.path", "Path to the proc file system to scrape metrics from").Default("/proc").String()

	_, err := kp.Parse(os.Args[1:])
	if err != nil {
		level.Error(logger).Log("msg", "failed to parse CLI options", "err", err)
		os.Exit(1)
	}

	registry := prometheus.DefaultRegisterer

	versionInfo := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "roger",
		Name:      "build_info",
		Help:      "Roger version information",
		ConstLabels: prometheus.Labels{
			"version":   Version,
			"revision":  Revision,
			"branch":    Branch,
			"goversion": runtime.Version(),
		},
	}, func() float64 { return 1 })
	registry.MustRegister(versionInfo)

	dnsmasqReader := roger.NewDnsmasqReader(new(dns.Client), *dnsServer, logger)
	registry.MustRegister(dnsmasqReader)

	netDevReader := roger.NewProcNetDevReader(*procPath, logger)
	if netDevReader.Exists() {
		registry.MustRegister(netDevReader)
	}

	connTrack := roger.NewProcNetStatReader(*procPath, "nf_conntrack", logger)
	if connTrack.Exists() {
		registry.MustRegister(connTrack)
	}

	arpCache := roger.NewProcNetStatReader(*procPath, "arp_cache", logger)
	if arpCache.Exists() {
		registry.MustRegister(arpCache)
	}

	index, err := template.New("index").Parse(indexTpt)
	if err != nil {
		level.Error(logger).Log("msg", "failed to parse index template", "err", err)
		os.Exit(1)
	}

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if err := index.Execute(w, *metricsPath); err != nil {
			level.Error(logger).Log("msg", "failed to render index", "err", err)
		}
	})

	if err := http.ListenAndServe(*webAddr, nil); err != nil {
		level.Error(logger).Log("err", err)
		os.Exit(1)
	}
}
