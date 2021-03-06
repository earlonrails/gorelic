package gorelic

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	metrics "github.com/yvasiyarov/go-metrics"
	nrpg "github.com/yvasiyarov/newrelic_platform_go"
)

const (
	// DefaultNewRelicPollInterval - how often we will report metrics to NewRelic.
	// Recommended values is 60 seconds
	DefaultNewRelicPollInterval = 60

	// DefaultGcPollIntervalInSeconds - how often we will get garbage collector run statistic
	// Default value is - every 10 seconds
	// During GC stat pooling - mheap will be locked, so be carefull changing this value
	DefaultGcPollIntervalInSeconds = 10

	// DefaultMemoryAllocatorPollIntervalInSeconds - how often we will get memory allocator statistic.
	// Default value is - every 60 seconds
	// During this process stoptheword() is called, so be carefull changing this value
	DefaultMemoryAllocatorPollIntervalInSeconds = 60

	//DefaultAgentGuid is plugin ID in NewRelic.
	//You should not change it unless you want to create your own plugin.
	DefaultAgentGuid = "com.github.earlonrails.GoRelic"

	//CurrentAgentVersion is plugin version
	CurrentAgentVersion = "0.0.1"

	//DefaultAgentName in NewRelic GUI. You can change it.
	DefaultAgentName = "Go daemon"
)

//Agent - is NewRelic agent implementation.
//Agent start separate go routine which will report data to NewRelic
type Agent struct {
	NewrelicName                string
	NewrelicLicense             string
	NewrelicPollInterval        int
	Verbose                     bool
	CollectGcStat               bool
	CollectMemoryStat           bool
	CollectHTTPStat             bool
	GCPollInterval              int
	MemoryAllocatorPollInterval int
	AgentGUID                   string
	AgentVersion                string
	plugin                      *nrpg.NewrelicPlugin
	HTTPTimer                   metrics.Timer
	HTTPRequestCounter          metrics.Counter
	HTTPRequestErrorCounter     metrics.Counter
	HTTPStatusCounters          map[int]metrics.Counter
	HTTPErrorCounters           map[int]metrics.Counter
	HTTPPathErrorCounters       map[string]map[int]metrics.Counter
	Tracer                      *Tracer
	CustomMetrics               []nrpg.IMetrica

	// All HTTP requests will be done using this client. Change it if you need
	// to use a proxy.
	Client http.Client
}

// NewAgent builds new Agent objects.
func NewAgent() *Agent {
	agent := &Agent{
		NewrelicName:                DefaultAgentName,
		NewrelicPollInterval:        DefaultNewRelicPollInterval,
		Verbose:                     false,
		CollectGcStat:               true,
		CollectMemoryStat:           true,
		GCPollInterval:              DefaultGcPollIntervalInSeconds,
		MemoryAllocatorPollInterval: DefaultMemoryAllocatorPollIntervalInSeconds,
		AgentGUID:                   DefaultAgentGuid,
		AgentVersion:                CurrentAgentVersion,
		Tracer:                      nil,
		CustomMetrics:               make([]nrpg.IMetrica, 0),
		HTTPPathErrorCounters:       make(map[string]map[int]metrics.Counter),
	}
	return agent
}

// our custom component
type resettableComponent struct {
	nrpg.IComponent
	requestCounter      metrics.Counter
	requestErrorCounter metrics.Counter
	statusCounters      map[int]metrics.Counter
	errorCounters       map[int]metrics.Counter
	errorPathCounters   map[string]map[int]metrics.Counter
}

// nrpg.IComponent interface implementation
func (c resettableComponent) ClearSentData() {
	c.IComponent.ClearSentData()
	c.requestCounter.Clear()
	c.requestErrorCounter.Clear()
	for _, counter := range c.statusCounters {
		counter.Clear()
	}
	for _, counter := range c.errorCounters {
		counter.Clear()
	}
	for _, counters := range c.errorPathCounters {
		for _, counter := range counters {
			counter.Clear()
		}
	}
}

type statusLoggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusLoggingResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

//WrapHTTPHandlerFunc  instrument HTTP handler functions to collect HTTP metrics
func (agent *Agent) WrapHTTPHandlerFunc(h tHTTPHandlerFunc, path string) tHTTPHandlerFunc {
	agent.registerHTTPPath(path)
	agent.CollectHTTPStat = true
	agent.initTimer()
	return func(w http.ResponseWriter, req *http.Request) {
		proxy := newHTTPHandlerFunc(h)
		proxy.timer = agent.HTTPTimer
		myW := &statusLoggingResponseWriter{w, 200}
		proxy.ServeHTTP(myW, req)
		agent.recordResponse(path, myW.status)
	}
}

//WrapHTTPHandler  instrument HTTP handler object to collect HTTP metrics
func (agent *Agent) WrapHTTPHandler(h http.Handler) http.Handler {
	agent.CollectHTTPStat = true
	agent.initTimer()

	proxy := newHTTPHandler(h)
	proxy.timer = agent.HTTPTimer
	return proxy
}

//AddCustomMetric adds metric to be collected periodically with NewrelicPollInterval interval
func (agent *Agent) AddCustomMetric(metric nrpg.IMetrica) {
	agent.CustomMetrics = append(agent.CustomMetrics, metric)
}

//Run initialize Agent instance and start harvest go routine
func (agent *Agent) Run() error {
	if agent.NewrelicLicense == "" {
		return errors.New("please, pass a valid newrelic license key")
	}

	var component nrpg.IComponent
	component = nrpg.NewPluginComponent(agent.NewrelicName, agent.AgentGUID, agent.Verbose)

	// Add default metrics and tracer.
	addRuntimeMericsToComponent(component)
	agent.Tracer = newTracer(component)

	// Check agent flags and add relevant metrics.
	if agent.CollectGcStat {
		addGCMetricsToComponent(component, agent.GCPollInterval)
		agent.debug(fmt.Sprintf("Init GC metrics collection. Poll interval %d seconds.", agent.GCPollInterval))
	}

	if agent.CollectMemoryStat {
		addMemoryMericsToComponent(component, agent.MemoryAllocatorPollInterval)
		agent.debug(fmt.Sprintf("Init memory allocator metrics collection. Poll interval %d seconds.", agent.MemoryAllocatorPollInterval))
	}

	if agent.CollectHTTPStat {
		agent.initTimer()
		agent.initStatusCounters()
		agent.initErrorCounters()

		addHTTPMericsToComponent(component, agent.HTTPTimer, agent.HTTPRequestCounter, agent.HTTPRequestErrorCounter)
		agent.debug(fmt.Sprintf("Init HTTP metrics collection."))

		component = &resettableComponent{component, agent.HTTPRequestCounter, agent.HTTPRequestErrorCounter, agent.HTTPStatusCounters, agent.HTTPErrorCounters, agent.HTTPPathErrorCounters}
		addHTTPStatusMetricsToComponent(component, agent.HTTPStatusCounters)
		agent.debug(fmt.Sprintf("Init HTTP status metrics collection."))

		addHTTPErrorMetricsToComponent(component, agent.HTTPErrorCounters)
		addHTTPPathErrorMetricsToComponent(component, agent.HTTPPathErrorCounters)
		agent.debug(fmt.Sprintf("Init HTTP status metrics collection."))
	}

	for _, metric := range agent.CustomMetrics {
		component.AddMetrica(metric)
		agent.debug(fmt.Sprintf("Init %s metric collection.", metric.GetName()))
	}

	// Init newrelic reporting plugin.
	agent.plugin = nrpg.NewNewrelicPlugin(agent.AgentVersion, agent.NewrelicLicense, agent.NewrelicPollInterval)
	agent.plugin.Client = agent.Client
	agent.plugin.Verbose = agent.Verbose

	// Add our metrics component to the plugin.
	agent.plugin.AddComponent(component)

	// Start reporting!
	go agent.plugin.Run()
	return nil
}

//RegisterHTTPPath registers a path for error status tracking
func (agent *Agent) registerHTTPPath(path string) {
	if agent.HTTPPathErrorCounters[path] == nil {
		agent.HTTPPathErrorCounters[path] = make(map[int]metrics.Counter)
	}
}

//RecordResponse increments different counters accordingly for an HTTP request
func (agent *Agent) recordResponse(path string, code int) {
	if agent.HTTPRequestCounter != nil {
		agent.HTTPRequestCounter.Inc(1)
	}

	if agent.HTTPStatusCounters != nil {
		agent.HTTPStatusCounters[code].Inc(1)
	}

	if httpErrors[code] {
		agent.HTTPRequestErrorCounter.Inc(1)
		agent.HTTPErrorCounters[code].Inc(1)
		if counters := agent.HTTPPathErrorCounters[path]; counters != nil {
			counters[code].Inc(1)
		}
	}
}

//Initialize global metrics.Timer object, used to collect HTTP metrics
func (agent *Agent) initTimer() {
	if agent.HTTPTimer == nil {
		agent.HTTPTimer = metrics.NewTimer()
	}
}

//Initialize metrics.Counters objects, used to collect HTTP statuses
func (agent *Agent) initStatusCounters() {
	agent.HTTPStatusCounters = make(map[int]metrics.Counter, len(httpStatuses))
	for _, statusCode := range httpStatuses {
		agent.HTTPStatusCounters[statusCode] = metrics.NewCounter()
	}
	agent.HTTPRequestCounter = metrics.NewCounter()
}

//Initialize metrics.Counters objects, used to collect HTTP statuses
func (agent *Agent) initErrorCounters() {
	agent.HTTPErrorCounters = make(map[int]metrics.Counter, len(httpErrors))
	for statusCode := range httpErrors {
		agent.HTTPErrorCounters[statusCode] = metrics.NewCounter()
	}
	for _, counters := range agent.HTTPPathErrorCounters {
		for statusCode := range httpErrors {
			counters[statusCode] = metrics.NewCounter()
		}
	}
	agent.HTTPRequestErrorCounter = metrics.NewCounter()
}

//Print debug messages
func (agent *Agent) debug(msg string) {
	if agent.Verbose {
		log.Println(msg)
	}
}
