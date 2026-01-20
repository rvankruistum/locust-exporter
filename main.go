package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	namespace string
	NameSpace *string
)

// Exporter structure
type Exporter struct {
	uri   string
	mutex sync.RWMutex
	fetch func(endpoint string) (io.ReadCloser, error)

	locustUp,
	locustUsers,
	locustFailRatio,
	locustCurrentResponseTimePercentileNinetyFifth,
	locustCurrentResponseTimePercentileFiftieth prometheus.Gauge
	locustRunning,
	locustWorkersCount,
	locustWorkersRunningCount,
	locustWorkersHatchingCount,
	locustWorkersMissingCount prometheus.Gauge
	locustNumRequests,
	locustNumFailures,
	locustAvgResponseTime,
	locustCurrentFailPerSec,
	locustWorkersDetail,
	locustMinResponseTime,
	locustMaxResponseTime,
	locustCurrentRps,
	locustMedianResponseTime,
	locustAvgContentLength,
	locustErrors *prometheus.GaugeVec
	locustResponseTimePercentile99,
	locustResponseTimePercentile95 *prometheus.GaugeVec
	totalScrapes prometheus.Counter
}

// NewExporter function
func NewExporter(uri string, timeout time.Duration) (*Exporter, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	var fetch func(endpoint string) (io.ReadCloser, error)
	switch u.Scheme {
	case "http", "https", "file":
		fetch = fetchHTTP(uri, timeout)
	default:
		return nil, fmt.Errorf("unsupported scheme: %q", u.Scheme)
	}

	return &Exporter{
		uri:   uri,
		fetch: fetch,
		locustRunning: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "running",
				Help:      "The current state of the execution (0 = STOPPED 1 = HATCHING 2 = RUNNING,).",
			},
		),
		locustUp: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "up",
				Help:      "The current health status of the server (1 = UP, 0 = DOWN).",
			},
		),
		locustUsers: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "users",
				Help:      "The current number of users.",
			},
		),
		locustWorkersCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "workers_count",
				Help:      "The current number of workers.",
			},
		),
		locustCurrentResponseTimePercentileNinetyFifth: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "requests",
				Name:      "current_response_time_percentile_95",
			},
		),
		locustCurrentResponseTimePercentileFiftieth: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "requests",
				Name:      "current_response_time_percentile_50",
			},
		),
		locustFailRatio: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "requests",
				Name:      "fail_ratio",
			},
		),
		locustWorkersRunningCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "workers_running_count",
				Help:      "The current number of running workers.",
			},
		),
		locustWorkersHatchingCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "workers_hatching_count",
				Help:      "The current number of hatching workers.",
			},
		),
		locustWorkersMissingCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "workers_missing_count",
				Help:      "The current number of missing workers.",
			},
		),
		locustWorkersDetail: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "worker",
				Name:      "detail",
				Help:      "The current status of a worker with user count",
			},
			[]string{"id", "state"},
		),
		locustNumRequests: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "requests",
				Name:      "num_requests",
			},
			[]string{"method", "name"},
		),
		locustNumFailures: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "requests",
				Name:      "num_failures",
			},
			[]string{"method", "name"},
		),
		locustAvgResponseTime: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "requests",
				Name:      "avg_response_time",
			},
			[]string{"method", "name"},
		),
		locustCurrentFailPerSec: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "requests",
				Name:      "current_fail_per_sec",
			},
			[]string{"method", "name"},
		),
		locustMinResponseTime: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "requests",
				Name:      "min_response_time",
			},
			[]string{"method", "name"},
		),
		locustMaxResponseTime: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "requests",
				Name:      "max_response_time",
			},
			[]string{"method", "name"},
		),
		locustCurrentRps: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "requests",
				Name:      "current_rps",
			},
			[]string{"method", "name"},
		),
		locustMedianResponseTime: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "requests",
				Name:      "median_response_time",
			},
			[]string{"method", "name"},
		),
		locustAvgContentLength: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "requests",
				Name:      "avg_content_length",
			},
			[]string{"method", "name"},
		),
		locustErrors: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "errors",
				Help:      "The current number of errors.",
			},
			[]string{"method", "name", "error"},
		),
		locustResponseTimePercentile95: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "requests",
				Name:      "response_time_percentile_95",
				Help:      "The 95th percentile of response time per endpoint.",
			},
			[]string{"method", "name"},
		),
		locustResponseTimePercentile99: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "requests",
				Name:      "response_time_percentile_99",
				Help:      "The 99th percentile of response time per endpoint.",
			},
			[]string{"method", "name"},
		),
		totalScrapes: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "total_scrapes",
				Help:      "The total number of scrapes.",
			},
		),
	}, nil
}

// Describe function of Exporter
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	ch <- e.locustUsers.Desc()
	ch <- e.locustWorkersCount.Desc()
	ch <- e.locustWorkersRunningCount.Desc()
	ch <- e.locustWorkersHatchingCount.Desc()
	ch <- e.locustWorkersMissingCount.Desc()
	ch <- e.locustUp.Desc()
	ch <- e.locustRunning.Desc()
	ch <- e.totalScrapes.Desc()
	ch <- e.locustFailRatio.Desc()
	ch <- e.locustCurrentResponseTimePercentileNinetyFifth.Desc()
	ch <- e.locustCurrentResponseTimePercentileFiftieth.Desc()

	e.locustNumRequests.Describe(ch)
	e.locustNumFailures.Describe(ch)
	e.locustAvgResponseTime.Describe(ch)
	e.locustCurrentFailPerSec.Describe(ch)
	e.locustMinResponseTime.Describe(ch)
	e.locustMaxResponseTime.Describe(ch)
	e.locustMedianResponseTime.Describe(ch)
	e.locustCurrentRps.Describe(ch)
	e.locustAvgContentLength.Describe(ch)
	e.locustResponseTimePercentile99.Describe(ch)
	e.locustResponseTimePercentile95.Describe(ch)
	e.locustErrors.Describe(ch)
	e.locustWorkersDetail.Describe(ch)
}

// Collect function of Exporter
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	up := e.scrape(ch)
	ch <- prometheus.MustNewConstMetric(e.locustUp.Desc(), prometheus.GaugeValue, up)
	e.locustNumRequests.Collect(ch)
	e.locustNumFailures.Collect(ch)
	e.locustAvgResponseTime.Collect(ch)
	e.locustCurrentFailPerSec.Collect(ch)
	e.locustMinResponseTime.Collect(ch)
	e.locustMaxResponseTime.Collect(ch)
	e.locustCurrentRps.Collect(ch)
	e.locustMedianResponseTime.Collect(ch)
	e.locustAvgContentLength.Collect(ch)
	e.locustResponseTimePercentile99.Collect(ch)
	e.locustResponseTimePercentile95.Collect(ch)
	e.locustErrors.Collect(ch)
	e.locustWorkersDetail.Collect(ch)
}

type locustStats struct {
	Stats []struct {
		Method                   string  `json:"method"`
		Name                     string  `json:"name"`
		NumRequests              int     `json:"num_requests"`
		NumFailures              int     `json:"num_failures"`
		AvgResponseTime          float64 `json:"avg_response_time"`
		CurrentFailPerSec        float64 `json:"current_fail_per_sec"`
		MinResponseTime          float64 `json:"min_response_time"`
		MaxResponseTime          float64 `json:"max_response_time"`
		CurrentRps               float64 `json:"current_rps"`
		MedianResponseTime       float64 `json:"median_response_time"`
		AvgContentLength         float64 `json:"avg_content_length"`
		ResponseTimePercentile99 float64 `json:"response_time_percentile_0.99"`
		ResponseTimePercentile95 float64 `json:"response_time_percentile_0.95"`
	} `json:"stats"`
	Errors []struct {
		Method      string `json:"method"`
		Name        string `json:"name"`
		Error       string `json:"error"`
		Occurrences int    `json:"occurrences"`
	} `json:"errors"`
	CurrentResponseTimePercentiles struct {
		CurrentResponseTimePercentileNinetyFifth float64 `json:"response_time_percentile_0.95"`
		CurrentResponseTimePercentileFiftieth    float64 `json:"response_time_percentile_0.5"`
	} `json:"current_response_time_percentiles"`
	TotalRps    float64 `json:"total_rps"`
	FailRatio   float64 `json:"fail_ratio"`
	WorkerCount int     `json:"worker_count,omitempty"`
	State       string  `json:"state"`
	UserCount   int     `json:"user_count"`
	Workers     []struct {
		Id        string `json:"id"`
		State     string `json:"state"`
		UserCount int    `json:"user_count"`
	} `json:"workers"`
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) (up float64) {
	e.totalScrapes.Inc()

	var locustStats locustStats

	body, err := e.fetch("/stats/requests")
	if err != nil {
		slog.Error("Can't scrape Pack", "error", err)
		return 0
	}
	defer func() { _ = body.Close() }()

	bodyAll, err := io.ReadAll(body)
	if err != nil {
		slog.Error("Failed to read response body", "error", err)
		return 0
	}

	if err := json.Unmarshal([]byte(bodyAll), &locustStats); err != nil {
		slog.Error("Failed to parse JSON response", "error", err)
		return 0
	}

	ch <- prometheus.MustNewConstMetric(e.locustUsers.Desc(), prometheus.GaugeValue, float64(locustStats.UserCount))
	ch <- prometheus.MustNewConstMetric(e.locustFailRatio.Desc(), prometheus.GaugeValue, float64(locustStats.FailRatio))
	ch <- prometheus.MustNewConstMetric(e.locustCurrentResponseTimePercentileNinetyFifth.Desc(), prometheus.GaugeValue, locustStats.CurrentResponseTimePercentiles.CurrentResponseTimePercentileNinetyFifth)
	ch <- prometheus.MustNewConstMetric(e.locustCurrentResponseTimePercentileFiftieth.Desc(), prometheus.GaugeValue, locustStats.CurrentResponseTimePercentiles.CurrentResponseTimePercentileFiftieth)
	ch <- prometheus.MustNewConstMetric(e.locustWorkersCount.Desc(), prometheus.GaugeValue, float64(len(locustStats.Workers)))
	ch <- prometheus.MustNewConstMetric(e.locustWorkersRunningCount.Desc(), prometheus.GaugeValue, countWorkersByState(locustStats, "running"))
	ch <- prometheus.MustNewConstMetric(e.locustWorkersHatchingCount.Desc(), prometheus.GaugeValue, countWorkersByState(locustStats, "hatching"))
	ch <- prometheus.MustNewConstMetric(e.locustWorkersMissingCount.Desc(), prometheus.GaugeValue, countWorkersByState(locustStats, "missing"))

	// Reset per-endpoint metrics to avoid stale label combinations
	for _, metric := range []*prometheus.GaugeVec{
		e.locustNumRequests, e.locustNumFailures, e.locustAvgResponseTime,
		e.locustCurrentFailPerSec, e.locustMinResponseTime, e.locustMaxResponseTime,
		e.locustCurrentRps, e.locustMedianResponseTime, e.locustAvgContentLength,
		e.locustResponseTimePercentile99, e.locustResponseTimePercentile95,
	} {
		metric.Reset()
	}

	for _, r := range locustStats.Stats {
		if r.Name != "Total" && r.Name != "//stats/requests" {
			e.locustNumRequests.WithLabelValues(r.Method, r.Name).Set(float64(r.NumRequests))
			e.locustNumFailures.WithLabelValues(r.Method, r.Name).Set(float64(r.NumFailures))
			e.locustAvgResponseTime.WithLabelValues(r.Method, r.Name).Set(r.AvgResponseTime)
			e.locustCurrentFailPerSec.WithLabelValues(r.Method, r.Name).Set(r.CurrentFailPerSec)
			e.locustMinResponseTime.WithLabelValues(r.Method, r.Name).Set(r.MinResponseTime)
			e.locustMaxResponseTime.WithLabelValues(r.Method, r.Name).Set(r.MaxResponseTime)
			e.locustCurrentRps.WithLabelValues(r.Method, r.Name).Set(r.CurrentRps)
			e.locustMedianResponseTime.WithLabelValues(r.Method, r.Name).Set(r.MedianResponseTime)
			e.locustAvgContentLength.WithLabelValues(r.Method, r.Name).Set(r.AvgContentLength)
			e.locustResponseTimePercentile99.WithLabelValues(r.Method, r.Name).Set(r.ResponseTimePercentile99)
			e.locustResponseTimePercentile95.WithLabelValues(r.Method, r.Name).Set(r.ResponseTimePercentile95)
		}
	}

	for _, r := range locustStats.Errors {
		e.locustErrors.WithLabelValues(r.Method, r.Name, r.Error).Set(float64(r.Occurrences))
	}

	for _, worker := range locustStats.Workers {
		e.locustWorkersDetail.WithLabelValues(worker.Id, worker.State).Set(float64(worker.UserCount))
	}

	var running = 0 //stopped

	switch locustStats.State {
	case "hatching":
		running = 1
	case "running":
		running = 2
	}

	ch <- prometheus.MustNewConstMetric(e.locustRunning.Desc(), prometheus.GaugeValue, float64(running))

	return 1
}

func fetchHTTP(uri string, timeout time.Duration) func(endpoint string) (io.ReadCloser, error) {
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := http.Client{
		Timeout:   timeout,
		Transport: tr,
	}

	return func(endpoint string) (io.ReadCloser, error) {
		resp, err := client.Get(uri + endpoint)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
		}
		return resp.Body, nil
	}
}

func countWorkersByState(stats locustStats, state string) float64 {
	var count = 0
	for _, worker := range stats.Workers {
		if worker.State == state {
			count++
		}
	}

	return float64(count)
}

// pushMetricsPeriodically pushes metrics to Pushgateway at specified interval
func pushMetricsPeriodically(ctx context.Context, url, job string, interval time.Duration, registry *prometheus.Registry) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Do an immediate push on startup
	pusher := push.New(url, job).Gatherer(registry)
	if err := pusher.Push(); err != nil {
		slog.Error("Failed to push metrics to Pushgateway on startup", "error", err)
	} else {
		slog.Info("Successfully pushed metrics to Pushgateway on startup")
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping metrics push to Pushgateway")
			return
		case <-ticker.C:
			pusher := push.New(url, job).Gatherer(registry)
			if err := pusher.Push(); err != nil {
				slog.Error("Failed to push metrics to Pushgateway", "error", err)
			} else {
				slog.Debug("Successfully pushed metrics to Pushgateway")
			}
		}
	}
}

func main() {
	var (
		listenAddress       = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9646").Envar("LOCUST_EXPORTER_WEB_LISTEN_ADDRESS").String()
		metricsPath         = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").Envar("LOCUST_EXPORTER_WEB_TELEMETRY_PATH").String()
		uri                 = kingpin.Flag("locust.uri", "URI of Locust.").Default("http://localhost:8089").Envar("LOCUST_EXPORTER_URI").String()
		NameSpace           = kingpin.Flag("locust.namespace", "Namespace for prometheus metrics.").Default("locust").Envar("LOCUST_METRIC_NAMESPACE").String()
		timeout             = kingpin.Flag("locust.timeout", "Scrape timeout").Default("5s").Envar("LOCUST_EXPORTER_TIMEOUT").Duration()
		pushgatewayURL      = kingpin.Flag("pushgateway.url", "URL of Pushgateway. If empty, push mode is disabled.").Default("").Envar("LOCUST_EXPORTER_PUSHGATEWAY_URL").String()
		pushgatewayJob      = kingpin.Flag("pushgateway.job", "Job label for Pushgateway.").Default("locust").Envar("LOCUST_EXPORTER_PUSHGATEWAY_JOB").String()
		pushgatewayInterval = kingpin.Flag("pushgateway.interval", "Interval for pushing metrics to Pushgateway.").Default("10s").Envar("LOCUST_EXPORTER_PUSHGATEWAY_INTERVAL").Duration()
	)

	kingpin.Version(version.Print("locust_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	namespace = *NameSpace
	slog.Info("Starting locust_exporter", "version", version.Info())
	slog.Info("Build context", "context", version.BuildContext())

	// Create custom registry
	reg := prometheus.NewRegistry()

	exporter, err := NewExporter(*uri, *timeout)
	if err != nil {
		slog.Error("Failed to create exporter", "error", err)
		os.Exit(1)
	}
	reg.MustRegister(exporter)
	reg.MustRegister(collectors.NewBuildInfoCollector())

	// If pushgateway is configured, start pusher
	var pushCtx context.Context
	var pushCancel context.CancelFunc
	var pushWg sync.WaitGroup
	if *pushgatewayURL != "" {
		slog.Info("Pushgateway enabled",
			"url", *pushgatewayURL,
			"job", *pushgatewayJob,
			"interval", *pushgatewayInterval)

		pushCtx, pushCancel = context.WithCancel(context.Background())
		defer pushCancel()

		pushWg.Add(1)
		go func() {
			defer pushWg.Done()
			pushMetricsPeriodically(pushCtx, *pushgatewayURL, *pushgatewayJob, *pushgatewayInterval, reg)
		}()
	} else {
		slog.Debug("Pushgateway disabled (no URL configured)")
	}

	http.Handle(*metricsPath, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	http.HandleFunc("/quitquitquit", func(http.ResponseWriter, *http.Request) { os.Exit(0) })
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><head><title>Locust Exporter</title></head><body><h1>Locust Exporter</h1><p><a href='` + *metricsPath + `'>Metrics</a></p></body></html>`))
	})

	// Setup signal handling for graceful shutdown (after pushgateway is started)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Handle graceful shutdown
	go func() {
		<-sigChan
		slog.Info("Received shutdown signal")
		if pushCancel != nil {
			pushCancel()
			slog.Info("Waiting for pushgateway goroutine to finish...")
			pushWg.Wait()
			slog.Info("Pushgateway goroutine stopped")
		}
		os.Exit(0)
	}()

	slog.Info("Listening on", "address", *listenAddress)
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		slog.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}
