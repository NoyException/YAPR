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

type MetricsRecorder interface {
	IncRequestTotal(pod string, uri string)
	IncAddPodTotal(service string, pod string)
	IncRemovePodTotal(service string, pod string)
	IncHangPodTotal(service string, pod string)
	ObserveRequestDuration(target string, duration float64)
	ObserveResolverDuration(duration float64)
	//ObserveBalancerDuration(duration float64)
	ObserveSelectorDuration(strategy string, duration float64)
	IncUpdateSelectorCnt(strategy string)
}

type MetricsRecorderImpl struct {
	requestTotal     *prometheus.CounterVec
	addPodTotal      *prometheus.CounterVec
	removePodTotal   *prometheus.CounterVec
	hangPodTotal     *prometheus.CounterVec
	requestDuration  *prometheus.SummaryVec
	resolverDuration *prometheus.SummaryVec
	//balancerDuration *prometheus.SummaryVec
	selectorDuration  *prometheus.SummaryVec
	updateSelectorCnt *prometheus.CounterVec
}

func newMetricsRecorder() MetricsRecorder {
	return &MetricsRecorderImpl{
		requestTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "request_total",
			Help: "The total number of requests",
		}, []string{"pod", "uri"}),

		addPodTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "add_pod_total",
			Help: "The total number of pods added",
		}, []string{"service", "pod"}),

		removePodTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "remove_pod_total",
			Help: "The total number of pods removed",
		}, []string{"service", "pod"}),

		hangPodTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "hang_pod_total",
			Help: "The total number of pods hang",
		}, []string{"service", "pod"}),

		requestDuration: promauto.NewSummaryVec(prometheus.SummaryOpts{
			Name:       "request_duration",
			Help:       "The duration between request and response",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}, []string{"target"}),

		resolverDuration: promauto.NewSummaryVec(prometheus.SummaryOpts{
			Name:       "resolver_duration",
			Help:       "The duration of resolver",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}, []string{}),

		//balancerDuration = promauto.NewSummaryVec(prometheus.SummaryOpts{
		//	name:       "balancer_duration",
		//	Help:       "The duration of balancer",
		//	Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		//}, []string{})

		selectorDuration: promauto.NewSummaryVec(prometheus.SummaryOpts{
			Name:       "selector_duration",
			Help:       "The duration of selector",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}, []string{"strategy"}),

		updateSelectorCnt: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "update_selector_cnt",
			Help: "The total number of selector updated",
		}, []string{"strategy"}),
	}
}

func (m *MetricsRecorderImpl) IncRequestTotal(pod string, uri string) {
	m.requestTotal.WithLabelValues(pod, uri).Inc()
}

func (m *MetricsRecorderImpl) IncAddPodTotal(service string, pod string) {
	m.addPodTotal.WithLabelValues(service, pod).Inc()
}

func (m *MetricsRecorderImpl) IncRemovePodTotal(service string, pod string) {
	m.removePodTotal.WithLabelValues(service, pod).Inc()
}

func (m *MetricsRecorderImpl) IncHangPodTotal(service string, pod string) {
	m.hangPodTotal.WithLabelValues(service, pod).Inc()
}

func (m *MetricsRecorderImpl) ObserveRequestDuration(target string, duration float64) {
	m.requestDuration.WithLabelValues(target).Observe(duration)
}

func (m *MetricsRecorderImpl) ObserveResolverDuration(duration float64) {
	m.resolverDuration.WithLabelValues().Observe(duration)
}

//func (m *MetricsRecorderImpl) ObserveBalancerDuration(duration float64) {
//	m.balancerDuration.WithLabelValues().Observe(duration)
//}

func (m *MetricsRecorderImpl) ObserveSelectorDuration(strategy string, duration float64) {
	m.selectorDuration.WithLabelValues(strategy).Observe(duration)
}

func (m *MetricsRecorderImpl) IncUpdateSelectorCnt(strategy string) {
	m.updateSelectorCnt.WithLabelValues(strategy).Inc()
}

type MetricsRecorderMock struct {
}

func (m *MetricsRecorderMock) IncRequestTotal(pod string, uri string) {
}

func (m *MetricsRecorderMock) IncAddPodTotal(service string, pod string) {
}

func (m *MetricsRecorderMock) IncRemovePodTotal(service string, pod string) {
}

func (m *MetricsRecorderMock) IncHangPodTotal(service string, pod string) {
}

func (m *MetricsRecorderMock) ObserveRequestDuration(target string, duration float64) {
}

func (m *MetricsRecorderMock) ObserveResolverDuration(duration float64) {
}

//func (m *MetricsRecorderMock) ObserveBalancerDuration(duration float64) {
//}

func (m *MetricsRecorderMock) ObserveSelectorDuration(strategy string, duration float64) {
}

func (m *MetricsRecorderMock) IncUpdateSelectorCnt(strategy string) {
}

var recorder MetricsRecorder

func Init(port int, mock bool) {
	if mock {
		recorder = &MetricsRecorderMock{}
	} else {
		recorder = newMetricsRecorder()
		http.Handle("/metrics", promhttp.Handler())
	}
	logger.Infof("metrics server started at :%d", port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}

func IncRequestTotal(pod string, uri string) {
	recorder.IncRequestTotal(pod, uri)
}

func IncAddPodTotal(service string, pod string) {
	recorder.IncAddPodTotal(service, pod)
}

func IncRemovePodTotal(service string, pod string) {
	recorder.IncRemovePodTotal(service, pod)
}

func IncHangPodTotal(service string, pod string) {
	recorder.IncHangPodTotal(service, pod)
}

func ObserveRequestDuration(target string, duration float64) {
	recorder.ObserveRequestDuration(target, duration)
}

func ObserveResolverDuration(duration float64) {
	recorder.ObserveResolverDuration(duration)
}

//func ObserveBalancerDuration(duration float64) {
//	recorder.ObserveBalancerDuration(duration)
//}

func ObserveSelectorDuration(strategy string, duration float64) {
	recorder.ObserveSelectorDuration(strategy, duration)
}

func IncUpdateSelectorCnt(strategy string) {
	recorder.IncUpdateSelectorCnt(strategy)
}
