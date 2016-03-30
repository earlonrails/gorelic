package gorelic

import (
	"net/http"
	"time"

	metrics "github.com/yvasiyarov/go-metrics"
	"github.com/yvasiyarov/newrelic_platform_go"
)

type tHTTPHandlerFunc func(http.ResponseWriter, *http.Request)
type tHTTPHandler struct {
	originalHandler     http.Handler
	originalHandlerFunc tHTTPHandlerFunc
	isFunc              bool
	timer               metrics.Timer
}

var httpTimer metrics.Timer

func newHTTPHandlerFunc(h tHTTPHandlerFunc) *tHTTPHandler {
	return &tHTTPHandler{
		isFunc:              true,
		originalHandlerFunc: h,
	}
}
func newHTTPHandler(h http.Handler) *tHTTPHandler {
	return &tHTTPHandler{
		isFunc:          false,
		originalHandler: h,
	}
}

func (handler *tHTTPHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	defer handler.timer.UpdateSince(startTime)

	if handler.isFunc {
		handler.originalHandlerFunc(w, req)
	} else {
		handler.originalHandler.ServeHTTP(w, req)
	}
}

func addHTTPMericsToComponent(component newrelic_platform_go.IComponent, timer metrics.Timer, reqCounter metrics.Counter, errCounter metrics.Counter) {
	rate1 := &timerRate1Metrica{
		baseTimerMetrica: &baseTimerMetrica{
			name:       "http/throughput/1minute",
			units:      "rps",
			dataSource: timer,
		},
	}
	component.AddMetrica(rate1)

	rateMean := &timerRateMeanMetrica{
		baseTimerMetrica: &baseTimerMetrica{
			name:       "http/throughput/rateMean",
			units:      "rps",
			dataSource: timer,
		},
	}
	component.AddMetrica(rateMean)

	responseTimeMean := &timerMeanMetrica{
		baseTimerMetrica: &baseTimerMetrica{
			name:       "http/responseTime/mean",
			units:      "ms",
			dataSource: timer,
		},
	}
	component.AddMetrica(responseTimeMean)

	responseTimeMax := &timerMaxMetrica{
		baseTimerMetrica: &baseTimerMetrica{
			name:       "http/responseTime/max",
			units:      "ms",
			dataSource: timer,
		},
	}
	component.AddMetrica(responseTimeMax)

	responseTimeMin := &timerMinMetrica{
		baseTimerMetrica: &baseTimerMetrica{
			name:       "http/responseTime/min",
			units:      "ms",
			dataSource: timer,
		},
	}
	component.AddMetrica(responseTimeMin)

	responseTimePercentile75 := &timerPercentile75Metrica{
		baseTimerMetrica: &baseTimerMetrica{
			name:       "http/responseTime/percentile75",
			units:      "ms",
			dataSource: timer,
		},
	}
	component.AddMetrica(responseTimePercentile75)

	responseTimePercentile90 := &timerPercentile90Metrica{
		baseTimerMetrica: &baseTimerMetrica{
			name:       "http/responseTime/percentile90",
			units:      "ms",
			dataSource: timer,
		},
	}
	component.AddMetrica(responseTimePercentile90)

	responseTimePercentile95 := &timerPercentile95Metrica{
		baseTimerMetrica: &baseTimerMetrica{
			name:       "http/responseTime/percentile95",
			units:      "ms",
			dataSource: timer,
		},
	}
	component.AddMetrica(responseTimePercentile95)

	component.AddMetrica(&counterByStatusMetrica{
		counter: reqCounter,
		name:    "http/requests",
		units:   "count",
	})

	component.AddMetrica(&errorRateMetrica{
		requestCounter: reqCounter,
		errorCounter:   errCounter,
		name:           "http/errorRate",
		units:          "value",
	})
}

// New metrica collector - counter per each http status code.
type errorRateMetrica struct {
	requestCounter metrics.Counter
	errorCounter   metrics.Counter
	name           string
	units          string
}

// metrics.IMetrica interface implementation.
func (m *errorRateMetrica) GetName() string { return m.name }

func (m *errorRateMetrica) GetUnits() string { return m.units }

func (m *errorRateMetrica) GetValue() (float64, error) {
	if m.requestCounter.Count() == 0 {
		return 0, nil
	}
	return float64(m.errorCounter.Count()) / float64(m.requestCounter.Count()), nil
}
