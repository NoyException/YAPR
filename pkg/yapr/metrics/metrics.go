package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	NodeTypeWhateverServer = "whateverSvr"
)

var (
	globalLabels   = []string{"node_type", "node_addr"}
	durationLabels = []string{"duration_key"}

	// 某个计数器指标
	whateverTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "whatever_total",
		Help: "The total number of whatever",
	}, globalLabels)

	whateverDuration = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Name:       "whatever_duration",
		Help:       "The duration of whatever",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, globalLabels)
)

func IncWhateverTotal(nodeAddr string) {
	whateverTotal.WithLabelValues(NodeTypeWhateverServer, nodeAddr).Inc()
}

func ObserveWhateverDuration(durationKey string, duration float64) {
	whateverDuration.WithLabelValues(durationKey).Observe(duration)
}
