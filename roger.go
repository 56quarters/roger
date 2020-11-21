package main

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

const (
	BufSize         = 100
	RunCheckFreq    = 5 * time.Second
	UpdateCheckFreq = 1 * time.Second
	StatusOk        = "ok"
	StatusErr       = "err"
)

type CheckResult struct {
	CheckName string      `json:"check_name"`
	Status    string      `json:"status"`
	Details   interface{} `json:"details"`
}

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

type CheckState struct {
	lock  *sync.Mutex
	state CheckResult
	queue <-chan CheckResult
}

func NewCheckState(lock *sync.Mutex, queue <-chan CheckResult) CheckState {
	return CheckState{
		lock:  lock,
		state: CheckResult{},
		queue: queue,
	}
}

func (s *CheckState) Update() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.state = <-s.queue
}

func (s *CheckState) Print() {
	s.lock.Lock()
	defer s.lock.Unlock()

	bytes, err := json.Marshal(s.state)
	if err != nil {
		log.Warn("Couldn't serialize result to JSON: %s", err)
	} else {
		fmt.Printf("%s\n", string(bytes))
	}
}

func RunChecks(queue chan<- CheckResult, check HealthCheck) {
	res, err := check.Refresh()
	if err != nil {
		log.Warn("Couldn't refresh check results: %s", err)
	} else {
		queue <- res
	}
}

func main() {
	log.SetFormatter(&log.JSONFormatter{})

	cpu := NewCpuHealthCheck()
	disk := NewDiskHealthCheck()
	composite := NewCompositeHealthCheck([]HealthCheck{cpu, disk})
	queue := make(chan CheckResult, BufSize)
	state := NewCheckState(&sync.Mutex{}, queue)

	runCheckTicker := time.NewTicker(RunCheckFreq)
	updateCheckTicker := time.NewTicker(UpdateCheckFreq)

	defer runCheckTicker.Stop()
	defer updateCheckTicker.Stop()

	for {
		select {
		case <-updateCheckTicker.C:
			go func() {
				state.Update()
				state.Print()
			}()
		case <-runCheckTicker.C:
			go RunChecks(queue, composite)
		}
	}
}
