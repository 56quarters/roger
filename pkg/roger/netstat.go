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
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// The "entries" field for the various /proc/net/stat metrics are shared
// for all CPUs and so they get special treatment in the way they are summed
// or not summed compared to other metrics.
const entriesHeader = "entries"

type ProcNetStatReader struct {
	subsystem    string
	path         string
	lock         sync.Mutex
	descriptions map[string]*prometheus.Desc
	logger       *slog.Logger
}

type NetStatResults struct {
	Values []ValueDesc
}

type ValueDesc struct {
	name     string
	val      uint64
	promType prometheus.ValueType
}

func NewProcNetStatReader(base string, variant string, logger *slog.Logger) *ProcNetStatReader {
	return &ProcNetStatReader{
		subsystem:    variant,
		path:         filepath.Join(base, "net", "stat", variant),
		lock:         sync.Mutex{},
		descriptions: make(map[string]*prometheus.Desc),
		logger:       logger,
	}
}

func (p *ProcNetStatReader) Describe(_ chan<- *prometheus.Desc) {
	// Unchecked collector. We don't return descriptors for the metrics that
	// the .Collect() method will return since they're constructed dynamically
	// based on the results of parsing the /proc/net/stats/$variant file.
}

func (p *ProcNetStatReader) Collect(ch chan<- prometheus.Metric) {
	res, err := p.ReadMetrics()
	if err != nil {
		p.logger.Error("failed to read net/stat metrics during collection", "path", p.path, "err", err)
		return
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	for _, v := range res.Values {
		desc, ok := p.descriptions[v.name]
		if !ok {
			desc = prometheus.NewDesc(v.name, fmt.Sprintf("generated from %s", p.path), nil, nil)
			p.descriptions[v.name] = desc
		}

		ch <- prometheus.MustNewConstMetric(desc, v.promType, float64(v.val))
	}
}

func (p *ProcNetStatReader) Exists() bool {
	if _, err := os.Stat(p.path); os.IsNotExist(err) {
		return false
	}

	return true
}

func (p *ProcNetStatReader) ReadMetrics() (*NetStatResults, error) {
	f, err := os.Open(p.path)
	if err != nil {
		return nil, err
	}

	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Scan()
	headers := strings.Fields(scanner.Text())
	parsed := make(map[string]ValueDesc)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		p.parseConnTrackValues(parsed, headers, parts)
	}

	parsedValues := make([]ValueDesc, 0, len(parsed))
	for _, v := range parsed {
		parsedValues = append(parsedValues, v)
	}
	return &NetStatResults{Values: parsedValues}, nil
}

func (p *ProcNetStatReader) parseConnTrackValues(parsed map[string]ValueDesc, headers []string, values []string) {
	for i := 0; i < len(headers); i++ {
		header := strings.ToLower(headers[i])
		name := prometheus.BuildFQName("roger", p.subsystem, header)
		val, err := strconv.ParseUint(values[i], 16, 64)

		if err != nil {
			p.logger.Warn("failed to parse value", "name", name, "value", values[i], "err", err)
			continue
		}

		existing, ok := parsed[name]
		if !ok {
			// The "entries" metrics for each of the /proc/net/stat files represents entries in
			// some sort of table that can go up or down and hence must be a gauge. The rest of
			// the values are counters
			var promType prometheus.ValueType
			if header == entriesHeader {
				promType = prometheus.GaugeValue
			} else {
				promType = prometheus.CounterValue
			}

			existing = ValueDesc{
				name:     name,
				val:      val,
				promType: promType,
			}

			parsed[name] = existing
		} else if header != entriesHeader {
			// The "entries" metrics for each CPU actually represents the total number of entries
			// in the table, it is shared across all CPUs. We only sum up the values here if the
			// metric is actually unique to each CPU (core, hyper-thread, etc)
			existing.val += val
		}

		parsed[name] = existing
	}
}
