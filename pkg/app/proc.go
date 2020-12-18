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

const interfaceLabel = "interface"

type ProcReader struct {
	path         string
	lock         sync.Mutex
	descriptions map[string]*prometheus.Desc
}

type NetInterfaceResults struct {
	InterfaceName string
	MetricValues  map[string]uint64
}

func NewProcReader(path string) ProcReader {
	return ProcReader{
		path:         path,
		lock:         sync.Mutex{},
		descriptions: make(map[string]*prometheus.Desc),
	}
}

func (p *ProcReader) Describe(_ chan<- *prometheus.Desc) {
	// Unchecked collector. We don't return descriptors for the metrics that
	// the .Collect() method will return since they're constructed dynamically
	// based on the results of parsing the /proc/net/dev file.
}

func (p *ProcReader) Collect(ch chan<- prometheus.Metric) {
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
				desc = metricDesc(k)
				p.descriptions[k] = desc
			}

			ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, float64(v), metrics.InterfaceName)
		}
	}
}

func (p *ProcReader) ReadMetrics() ([]NetInterfaceResults, error) {
	netDev := filepath.Join(p.path, "net", "dev")
	f, err := os.Open(netDev)
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

		appendValues(metrics, "roger", "net_rx", rxHeaders, rxVals)
		appendValues(metrics, "roger", "net_tx", txHeaders, txVals)

		res = append(res, NetInterfaceResults{
			InterfaceName: iface,
			MetricValues:  metrics,
		})
	}

	return res, nil
}

func appendValues(metrics map[string]uint64, namespace string, subsystem string, headers []string, values []string) {
	for i := 0; i < len(headers); i++ {
		name := prometheus.BuildFQName(namespace, subsystem, headers[i])
		val, err := strconv.ParseUint(values[i], 10, 64)

		if err != nil {
			Log.Warnf("Failed to parse value for %s: %s", name, err)
			continue
		}

		metrics[name] = val
	}
}

func metricDesc(name string) *prometheus.Desc {
	return prometheus.NewDesc(name, "", []string{interfaceLabel}, nil)
}
