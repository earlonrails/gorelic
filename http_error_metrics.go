package gorelic

import (
	"fmt"

	metrics "github.com/yvasiyarov/go-metrics"
	nrpg "github.com/yvasiyarov/newrelic_platform_go"
)

// addHTTPStatusMetricsToComponent initializes counter metrics for all http statuses and adds them to the component.
func addHTTPErrorMetricsToComponent(component nrpg.IComponent, statusCounters map[int]metrics.Counter) {
	for statusCode, counter := range statusCounters {
		component.AddMetrica(&counterByStatusMetrica{
			counter: counter,
			name:    fmt.Sprintf("http/all/error/%d", statusCode),
			units:   "count",
		})
	}
}

// addPerPathHTTPStatusMetricsToComponent initializes counter metrics for all http statuses and adds them to the component.
func addHTTPPathErrorMetricsToComponent(component nrpg.IComponent, statusCounters map[string]map[int]metrics.Counter) {
	for path, counters := range statusCounters {
		for statusCode, counter := range counters {
			component.AddMetrica(&counterByStatusMetrica{
				counter: counter,
				name:    fmt.Sprintf("http/path/%v/error/%d", path, statusCode),
				units:   "count",
			})
		}
	}
}
