// Roger - DNS and network metrics exporter for Prometheus
//
// Copyright 2020 Nick Pillitteri
//
// Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
// http://www.apache.org/licenses/LICENSE-2.0> or the MIT license
// <LICENSE-MIT or http://opensource.org/licenses/MIT>, at your
// option. This file may not be copied, modified, or distributed
// except according to those terms.

package app

// read network stats from /proc

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type ProcNetDevReader struct {
	path         string
	lock         sync.Mutex
	descriptions map[string]*prometheus.Desc
}

type NetInterfaceResults struct {
	InterfaceName string
	MetricValues  map[string]uint64
}

func NewProcNetDevReader(base string) *ProcNetDevReader {
	return &ProcNetDevReader{
		path:         filepath.Join(base, "net", "dev"),
		lock:         sync.Mutex{},
		descriptions: make(map[string]*prometheus.Desc),
	}
}

func (p *ProcNetDevReader) Describe(_ chan<- *prometheus.Desc) {
	// Unchecked collector. We don't return descriptors for the metrics that
	// the .Collect() method will return since they're constructed dynamically
	// based on the results of parsing the /proc/net/dev file.
}

func (p *ProcNetDevReader) Collect(ch chan<- prometheus.Metric) {
	res, err := p.ReadMetrics()
	if err != nil {
		Log.Warnf("Failed to read metrics during collection: %s", err)
		return
	}

	// Locking since we're modifying our cache of metric descriptions as we emit
	// values for them (and collectors must be safe to be called concurrently)
	p.lock.Lock()
	defer p.lock.Unlock()

	for _, metrics := range res {
		for k, v := range metrics.MetricValues {
			desc, ok := p.descriptions[k]
			if !ok {
				desc = prometheus.NewDesc(k, fmt.Sprintf("generated from %s", p.path), []string{"interface"}, nil)
				p.descriptions[k] = desc
			}

			ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, float64(v), metrics.InterfaceName)
		}
	}
}

func (p *ProcNetDevReader) Exists() bool {
	if _, err := os.Stat(p.path); os.IsNotExist(err) {
		return false
	} else {
		return true
	}
}

func (p *ProcNetDevReader) ReadMetrics() ([]NetInterfaceResults, error) {
	f, err := os.Open(p.path)
	if err != nil {
		return nil, err
	}

	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Scan()
	scanner.Scan() // skip header line

	headerLine := scanner.Text()
	headerParts := strings.Split(headerLine, "|")

	if len(headerParts) != 3 {
		return nil, fmt.Errorf("unexpected header line format %s", headerLine)
	}

	rxHeaders := strings.Fields(headerParts[1])
	txHeaders := strings.Fields(headerParts[2])
	var res []NetInterfaceResults

	for {
		if !scanner.Scan() {
			break
		}

		line := scanner.Text()
		parts := strings.Fields(line)
		iface := strings.TrimRight(parts[0], ":")
		rxVals := parts[1 : len(rxHeaders)+1]
		txVals := parts[len(rxHeaders)+1:]
		metrics := make(map[string]uint64)

		appendNetDevValues(metrics, rxHeaders, rxVals, "net_rx")
		appendNetDevValues(metrics, txHeaders, txVals, "net_tx")

		res = append(res, NetInterfaceResults{
			InterfaceName: iface,
			MetricValues:  metrics,
		})
	}

	return res, nil
}

func appendNetDevValues(metrics map[string]uint64, headers []string, values []string, subsystem string) {
	for i := 0; i < len(headers); i++ {
		name := prometheus.BuildFQName("roger", subsystem, strings.ToLower(headers[i]))
		val, err := strconv.ParseUint(values[i], 10, 64)

		if err != nil {
			Log.Warnf("Failed to parse value for %s: %s", name, err)
			continue
		}

		metrics[name] = val
	}
}
