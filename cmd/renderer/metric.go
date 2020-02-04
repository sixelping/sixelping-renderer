package main

import "github.com/prometheus/client_golang/prometheus"

type ReceiverMetric struct {
	counterDesc *prometheus.Desc
	values      map[string]uint64
}

type MacIPPair struct {
	Mac, Ip string
}

type ReceiverPerClientMetric struct {
	counterDesc *prometheus.Desc
	values      map[MacIPPair]uint64
}

func (c *ReceiverMetric) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.counterDesc
}

func (c *ReceiverPerClientMetric) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.counterDesc
}

func (c *ReceiverMetric) Collect(ch chan<- prometheus.Metric) {
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
	c.values[mac] = value
}

func (c *ReceiverPerClientMetric) Set(mac string, ip string, value uint64) {
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
