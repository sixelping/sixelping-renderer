package main

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type ReceiverMetric struct {
	counterDesc *prometheus.Desc
	values      map[string]uint64
	mut         sync.Mutex
}

type MacIPPair struct {
	Mac, Ip string
}

type ReceiverPerClientMetric struct {
	counterDesc *prometheus.Desc
	values      map[MacIPPair]uint64
	mut         sync.Mutex
}

func (c *ReceiverMetric) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.counterDesc
}

func (c *ReceiverPerClientMetric) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.counterDesc
}

func (c *ReceiverMetric) Collect(ch chan<- prometheus.Metric) {
	c.mut.Lock()
	defer c.mut.Unlock()
	for k, v := range c.values {
		ch <- prometheus.MustNewConstMetric(
			c.counterDesc,
			prometheus.CounterValue,
			float64(v),
			k,
		)
	}
}

func (c *ReceiverPerClientMetric) Collect(ch chan<- prometheus.Metric) {
	c.mut.Lock()
	defer c.mut.Unlock()
	for k, v := range c.values {
		ch <- prometheus.MustNewConstMetric(
			c.counterDesc,
			prometheus.CounterValue,
			float64(v),
			k.Mac,
			k.Ip,
		)
	}
}

func (c *ReceiverMetric) Set(mac string, value uint64) {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.values[mac] = value
}

func (c *ReceiverPerClientMetric) Set(mac string, ip string, value uint64) {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.values[MacIPPair{mac, ip}] = value
}

func NewReceiverMetric(name string, help string) *ReceiverMetric {
	return &ReceiverMetric{
		counterDesc: prometheus.NewDesc(name, help, []string{"mac"}, nil),
		values:      make(map[string]uint64),
	}
}

func NewReceiverPerClientMetric(name string, help string) *ReceiverPerClientMetric {
	return &ReceiverPerClientMetric{
		counterDesc: prometheus.NewDesc(name, help, []string{"mac", "ip"}, nil),
		values:      make(map[MacIPPair]uint64),
	}
}
