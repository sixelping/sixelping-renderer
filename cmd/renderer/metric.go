package main

import "github.com/prometheus/client_golang/prometheus"

type ReceiverMetric struct {
	counterDesc *prometheus.Desc
	values      map[string]uint64
}

func (c *ReceiverMetric) Describe(ch chan<- *prometheus.Desc) {
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

func (c *ReceiverMetric) Set(mac string, value uint64) {
	c.values[mac] = value
}

func NewReceiverMetric(name string, help string) *ReceiverMetric {
	return &ReceiverMetric{
		counterDesc: prometheus.NewDesc(name, help, []string{"mac"}, nil),
		values:      make(map[string]uint64),
	}
}
