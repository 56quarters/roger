package main

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"time"
)

const (
	BufSize   = 100
	CheckFreq = 5 * time.Second
	ReadFreq  = 1 * time.Second
	StatusOk  = "ok"
	StatusErr = "err"
)

type HealthCheck interface {
	Refresh() (CheckResult, error)
	Name() string
}

type CpuHealthCheck struct{}

func NewCpuHealthCheck() CpuHealthCheck {
	return CpuHealthCheck{}
}

func (c CpuHealthCheck) Refresh() (CheckResult, error) {
	return CheckResult{
		Status:    StatusOk,
		CheckName: c.Name(),
		Details:   map[string]string{"cores": "4"},
	}, nil
}

func (c CpuHealthCheck) Name() string {
	return "cpu"
}

type DiskHealthCheck struct{}

func NewDiskHealthCheck() DiskHealthCheck {
	return DiskHealthCheck{}
}

func (c DiskHealthCheck) Refresh() (CheckResult, error) {
	return CheckResult{
		Status:    StatusErr,
		CheckName: c.Name(),
		Details:   map[string]string{"bytes_free": "1234"},
	}, nil
}

func (c DiskHealthCheck) Name() string {
	return "disk"
}

type CompositeHealthCheck struct {
	checks []HealthCheck
}

func NewCompositeHealthCheck(checks []HealthCheck) CompositeHealthCheck {
	return CompositeHealthCheck{
		checks: checks,
	}
}

func (c CompositeHealthCheck) Refresh() (CheckResult, error) {
	status := StatusOk
	var details []CheckResult

	for _, other := range c.checks {
		res, err := other.Refresh()
		if err != nil {
			log.Warn("Failed to run %s check: %s", other.Name(), err)
			continue
		}

		if res.Status != StatusOk {
			status = StatusErr
		}

		details = append(details, res)
	}

	return CheckResult{
		Status:    status,
		CheckName: c.Name(),
		Details:   details,
	}, nil
}

func (c CompositeHealthCheck) Name() string {
	return "composite"
}

type CheckResult struct {
	CheckName string      `json:"check_name"`
	Status    string      `json:"status"`
	Details   interface{} `json:"details"`
}

func RunChecks(queue chan<- CheckResult, check HealthCheck) {
	res, err := check.Refresh()
	if err != nil {
		log.Warn("Couldn't refresh check results: %s", err)
	} else {
		queue <- res
	}
}

func PrintCheckResults(queue <-chan CheckResult) {
	res := <-queue
	bytes, err := json.Marshal(res)
	if err != nil {
		log.Warn("Couldn't serialize result to JSON: %s", err)
	} else {
		fmt.Printf("%s\n", string(bytes))
	}
}

func main() {
	log.SetFormatter(&log.JSONFormatter{})

	cpu := NewCpuHealthCheck()
	disk := NewDiskHealthCheck()
	composite := NewCompositeHealthCheck([]HealthCheck{cpu, disk})
	queue := make(chan CheckResult, BufSize)

	checkTicker := time.NewTicker(CheckFreq)
	readTicker := time.NewTicker(ReadFreq)

	defer checkTicker.Stop()
	defer readTicker.Stop()

	for {
		select {
		case <-readTicker.C:
			go PrintCheckResults(queue)
		case <-checkTicker.C:
			go RunChecks(queue, composite)
		}
	}
}
