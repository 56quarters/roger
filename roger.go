package main

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"net/http"
	"sync"
	"time"
)

const (
	BufSize         = 100
	RunCheckFreq    = 5 * time.Second
	UpdateCheckFreq = 1 * time.Second
	StatusOk        = "ok"
	StatusErr       = "err"
	ContentType     = "application/json"
)

type CheckResult struct {
	CheckName string      `json:"check_name"`
	Status    string      `json:"status"`
	Details   interface{} `json:"details"`
}

func DefaultCheckResult() CheckResult {
	return CheckResult{
		CheckName: "none",
		Status:    StatusOk,
		Details:   map[string]string{},
	}
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
		state: DefaultCheckResult(),
		queue: queue,
	}
}

func (s *CheckState) Update() {
	res := <-s.queue

	// only take a lock after a result from the channel
	s.lock.Lock()
	defer s.lock.Unlock()
	s.state = res
}

func (s *CheckState) GetJson() ([]byte, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	bytes, err := json.Marshal(s.state)
	if err != nil {
		return nil, err
	}

	return bytes, nil
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

	go func() {
		for {
			select {
			case <-updateCheckTicker.C:
				go state.Update()
			case <-runCheckTicker.C:
				go RunChecks(queue, composite)
			}
		}
	}()

	http.HandleFunc("/-/healthy", func(w http.ResponseWriter, r *http.Request) {})
	http.HandleFunc("/-/ready", func(w http.ResponseWriter, r *http.Request) {})
	http.HandleFunc("/api/check", func(w http.ResponseWriter, r *http.Request) {
		res, err := state.GetJson()
		if err != nil {
			log.Warn("Unexpected error serializing state: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.Header().Set("content-type", ContentType)
			_, _ = w.Write(res)
		}
	})

	s := &http.Server{
		Addr:           ":8080",
		Handler:        nil,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Fatal(s.ListenAndServe())
}
