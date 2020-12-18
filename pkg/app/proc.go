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
	path    string
	lock    sync.Mutex
	metrics []NetInterfaceResults
}

type NetInterfaceResults struct {
	InterfaceName string
	MetricValues  map[string]uint64
}

func NewProcReader(path string) ProcReader {
	return ProcReader{path: path}
}

func appendValues(metrics map[string]uint64, namespace string, subsystem string, headers []string, values []string) {
	for i := 0; i < len(headers); i++ {
		name := prometheus.BuildFQName(namespace, subsystem, headers[i])
		val, err := strconv.ParseUint(values[i], 10, 64)

		if err != nil {
			Log.Warn("Failed to parse value for %s: %s", name, err)
			continue
		}

		metrics[name] = val
	}
}

func metricDesc(name string) *prometheus.Desc {
	return prometheus.NewDesc(name, "", []string{interfaceLabel}, nil)
}

func (p *ProcReader) Describe(ch chan<- *prometheus.Desc) {
	p.lock.Lock()
	defer p.lock.Unlock()

	for _, ifaceMetrics := range p.metrics {
		for k, _ := range ifaceMetrics.MetricValues {
			ch <- metricDesc(k)
		}
	}
}

func (p *ProcReader) Collect(ch chan<- prometheus.Metric) {
	p.lock.Lock()
	defer p.lock.Unlock()

	for _, ifaceMetrics := range p.metrics {
		for k, v := range ifaceMetrics.MetricValues {
			ch <- prometheus.MustNewConstMetric(metricDesc(k), prometheus.CounterValue, float64(v), ifaceMetrics.InterfaceName)
		}
	}
}

func (p *ProcReader) ReadMetrics() ([]NetInterfaceResults, error) {
	netDev := filepath.Join(p.path, "net", "dev")
	f, err := os.Open(netDev)

	if err != nil {
		return nil, err
	}
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

func (p *ProcReader) Update() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	res, err := p.ReadMetrics()
	if err != nil {
		return err
	}

	p.metrics = res
	return nil
}
