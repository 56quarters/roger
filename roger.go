package main

import (
	"fmt"
)

type HealthCheck interface {
	Refresh() (map[string]string, error)
}

type CpuHealthCheck struct {

}

func NewCpuHealthCheck() CpuHealthCheck {
	return CpuHealthCheck{}
}

func (c CpuHealthCheck) Refresh() (map[string]string, error) {
	return map[string]string{"status": "ok"}, nil
}

func main() {
	check := NewCpuHealthCheck()
	check.Refresh()
	fmt.Println("Roger roger, over and out")
}
