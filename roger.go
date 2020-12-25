// Roger - DNS and network metrics exporter for Prometheus
//
// Copyright 2020 Nick Pillitteri
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

	"github.com/56quarters/roger/pkg/app"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/alecthomas/kingpin.v2"
)

const indexTpt = `
<html>
<head><title>Roger Exporter</title></head>
<body>
<h1>Roger Exporter</h1>
<p><a href="{{ . }}">Metrics</a></p>
</body>
</html>
`

func main() {
	kp := kingpin.New(os.Args[0], "Roger: DNS and network metrics exporter for Prometheus")
	metricsPath := kp.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	webAddr := kp.Flag("web.listen-address", "Address and port to expose Prometheus metrics on").Default(":9779").String()
	dnsServer := kp.Flag("dns.server", "DNS server to export metrics for, including port").Default("127.0.0.1:53").String()
	procPath := kp.Flag("proc.path", "Path to the proc file system to scrape metrics from").Default("/proc").String()

	_, err := kp.Parse(os.Args[1:])
	if err != nil {
		app.Log.Fatal(err)
	}

	registry := prometheus.DefaultRegisterer

	dnsmasqReader := app.NewDnsmasqReader(new(dns.Client), *dnsServer)
	registry.MustRegister(dnsmasqReader)

	netDevReader := app.NewProcNetDevReader(*procPath)
	registry.MustRegister(netDevReader)

	// These metrics aren't guaranteed to exist
	connTrack := app.NewProcNetStatReader(*procPath, "nf_conntrack")
	if connTrack.Exists() {
		registry.MustRegister(connTrack)
	}

	// These metrics aren't guaranteed to exist
	arpCache := app.NewProcNetStatReader(*procPath, "arp_cache")
	if arpCache.Exists() {
		registry.MustRegister(arpCache)
	}

	index, err := template.New("index").Parse(indexTpt)
	if err != nil {
		app.Log.Fatal(err)
	}

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if err := index.Execute(w, *metricsPath); err != nil {
			app.Log.Errorf("Failed to render index: %s", err)
		}
	})
	app.Log.Error(http.ListenAndServe(*webAddr, nil))
}
