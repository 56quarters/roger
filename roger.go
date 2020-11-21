package main

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"sync"
)

const (
	BufSize = 100
)

type HealthCheck interface {
	Refresh() (map[string]string, error)
}

type CpuHealthCheck struct {
}

func NewCpuHealthCheck() CpuHealthCheck {
	return CpuHealthCheck{}
}

func (c *CpuHealthCheck) Refresh() (map[string]string, error) {
	return map[string]string{"status": "ok"}, nil
}

func SendResults(check HealthCheck, queue chan<- map[string]string, wg *sync.WaitGroup) {
	defer wg.Done()

	res, err := check.Refresh()
	if err != nil {
		log.Warn("Couldn't refresh check results: %s", err)
	} else {
		queue <- res
	}
}

func PrintCheckResults(queue <-chan map[string]string, wg *sync.WaitGroup) {
	defer wg.Done()

	res := <-queue
	bytes, err := json.Marshal(res)
	if err != nil {
		log.Warn("Couldn't serialize result to JSON: %s", err)
	} else {
		fmt.Printf("Roger roger, over and out: %s", string(bytes))
	}
}

func main() {
	log.SetFormatter(&log.JSONFormatter{})
	var wg sync.WaitGroup

	check := NewCpuHealthCheck()
	queue := make(chan map[string]string, BufSize)

	wg.Add(1)
	go SendResults(&check, queue, &wg)

	wg.Add(1)
	go PrintCheckResults(queue, &wg)

	wg.Wait()
}
