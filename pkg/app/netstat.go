package app

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

type ProcNetStatReader struct {
	subsystem    string
	path         string
	lock         sync.Mutex
	descriptions map[string]*prometheus.Desc
}

type NetStatResults struct {
	Values []ValueDesc
	Cpu    int
}

type ValueDesc struct {
	name     string
	val      uint64
	promType prometheus.ValueType
}

func NewProcNetStatReader(base string, variant string) *ProcNetStatReader {
	return &ProcNetStatReader{
		subsystem:    variant,
		path:         filepath.Join(base, "net", "stat", variant),
		lock:         sync.Mutex{},
		descriptions: make(map[string]*prometheus.Desc),
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
		Log.Warnf("Failed to read metrics during collection: %s", err)
		return
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	for i, metrics := range res {
		for _, v := range metrics.Values {
			desc, ok := p.descriptions[v.name]
			if !ok {
				desc = prometheus.NewDesc(v.name, fmt.Sprintf("generated from %s", p.path), []string{"cpu"}, nil)
				p.descriptions[v.name] = desc
			}

			ch <- prometheus.MustNewConstMetric(desc, v.promType, float64(v.val), strconv.Itoa(i))
		}
	}
}

func (p *ProcNetStatReader) Exists() bool {
	if _, err := os.Stat(p.path); os.IsNotExist(err) {
		return false
	} else {
		return true
	}
}

func (p *ProcNetStatReader) ReadMetrics() ([]NetStatResults, error) {
	f, err := os.Open(p.path)
	if err != nil {
		return nil, err
	}

	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Scan()
	headers := strings.Fields(scanner.Text())

	var res []NetStatResults
	for cpu := 0; ; cpu++ {
		if !scanner.Scan() {
			break
		}

		line := scanner.Text()
		parts := strings.Fields(line)

		res = append(res, NetStatResults{
			Values: p.parseConnTrackValues(headers, parts),
			Cpu:    cpu,
		})
	}

	return res, nil
}

func (p *ProcNetStatReader) parseConnTrackValues(headers []string, values []string) []ValueDesc {
	out := make([]ValueDesc, len(headers))

	for i := 0; i < len(headers); i++ {
		header := strings.ToLower(headers[i])
		name := prometheus.BuildFQName("roger", p.subsystem, header)
		val, err := strconv.ParseUint(values[i], 16, 64)

		if err != nil {
			Log.Warnf("Failed to parse value for %s: %s", name, err)
			continue
		}

		// The "entries" metrics for each of the /proc/net/stat files represents entries in
		// some sort of table that can go up or down and hence must be a gauge. The rest of
		// the values are counters
		promType := prometheus.CounterValue
		if header == "entries" {
			promType = prometheus.GaugeValue
		}

		out[i] = ValueDesc{
			name:     name,
			val:      val,
			promType: promType,
		}
	}

	return out
}
