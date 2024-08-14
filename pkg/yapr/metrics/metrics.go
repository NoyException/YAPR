package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	_ "net/http/pprof"
	"noy/router/pkg/yapr/logger"
	"strconv"
)

var (
	requestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "request_total",
		Help: "The total number of requests",
	}, []string{"pod", "uri"})

	addPodTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "add_pod_total",
		Help: "The total number of pods added",
	}, []string{"service", "pod"})

	removePodTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "remove_pod_total",
		Help: "The total number of pods removed",
	}, []string{"service", "pod"})

	hangPodTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "hang_pod_total",
		Help: "The total number of pods hang",
	}, []string{"service", "pod"})

	gRPCDuration = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Name:       "grpc_duration",
		Help:       "The duration of gRPC",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"target"})

	resolverDuration = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Name:       "resolver_duration",
		Help:       "The duration of resolver",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{})

	//balancerDuration = promauto.NewSummaryVec(prometheus.SummaryOpts{
	//	name:       "balancer_duration",
	//	Help:       "The duration of balancer",
	//	Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	//}, []string{})

	selectorDuration = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Name:       "selector_duration",
		Help:       "The duration of selector",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"strategy"})
)

func Init(port int) {
	http.Handle("/metrics", promhttp.Handler())
	logger.Infof("metrics server started at :%d", port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}

func IncRequestTotal(pod string, uri string) {
	requestTotal.WithLabelValues(pod, uri).Inc()

}

func IncAddPodTotal(service string, pod string) {
	addPodTotal.WithLabelValues(service, pod).Inc()
}

func IncRemovePodTotal(service string, pod string) {
	removePodTotal.WithLabelValues(service, pod).Inc()
}

func IncHangPodTotal(service string, pod string) {
	hangPodTotal.WithLabelValues(service, pod).Inc()
}

func ObserveGRPCDuration(target string, duration float64) {
	gRPCDuration.WithLabelValues(target).Observe(duration)
}

func ObserveResolverDuration(duration float64) {
	resolverDuration.WithLabelValues().Observe(duration)
}

//func ObserveBalancerDuration(duration float64) {
//	balancerDuration.WithLabelValues().Observe(duration)
//}

func ObserveSelectorDuration(strategy string, duration float64) {
	selectorDuration.WithLabelValues(strategy).Observe(duration)
}
