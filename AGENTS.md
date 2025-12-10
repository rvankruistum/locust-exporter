# Agent Guidelines for locust_exporter

## What This Project Is

**locust_exporter** is a Prometheus exporter for [Locust](https://locust.io/), a popular open-source load testing tool. It bridges the gap between Locust's load testing metrics and Prometheus monitoring infrastructure.

### Problems It Solves
1. **Monitoring Load Tests**: Exposes Locust's real-time load testing metrics in Prometheus format
2. **Visualization**: Enables Grafana dashboards to visualize load test performance (dashboard ID: 11985)
3. **Observability**: Integrates load testing data into existing Prometheus/Grafana monitoring stacks
4. **Worker Tracking**: Monitors distributed Locust worker states and health

### Key Metrics Exposed
- `locust_running` - Execution state (STOPPED=0, HATCHING=1, RUNNING=2)
- `locust_users` - Current number of simulated users
- `locust_workers_*` - Worker counts and states
- `locust_requests_*` - Request metrics per method/endpoint (including p95/p99 percentiles)
- `locust_errors` - Error occurrences by type

## Build/Test/Lint Commands
- **Build**: `make build` or `go build -o locust_exporter .`
- **Test**: `make test` (runs `go test ./...` - 80% coverage, 8 suites, 19 sub-tests)
- **Lint**: `make lint` (uses golangci-lint v2.7.0)
- **Format**: `make format` or `gofmt -w .`
- **Vet**: `make vet` or `go vet ./...`
- **Run locally**: `go run main.go --locust.uri=http://localhost:8089`
- **Docker build**: `docker build -t locust_exporter .` (multi-stage, 42 lines)

## Configuration

### Command-line Flags
- `--locust.uri`: Locust web interface URL (default: http://localhost:8089)
- `--locust.timeout`: Request timeout (default: 5s)
- `--web.listen-address`: Exporter port (default: :9646)
- `--web.telemetry-path`: Metrics endpoint (default: /metrics)
- `--locust.namespace`: Prometheus namespace (default: locust)

### Environment Variables
```bash
LOCUST_EXPORTER_URI                  # Locust URL
LOCUST_EXPORTER_TIMEOUT              # Request timeout
LOCUST_EXPORTER_WEB_LISTEN_ADDRESS   # Listen address
LOCUST_EXPORTER_WEB_TELEMETRY_PATH   # Metrics path
LOCUST_METRIC_NAMESPACE              # Prometheus namespace
LOCUST_EXPORTER_PUSHGATEWAY_URL      # Pushgateway URL (empty = disabled)
LOCUST_EXPORTER_PUSHGATEWAY_JOB      # Pushgateway job label (default: locust)
LOCUST_EXPORTER_PUSHGATEWAY_INTERVAL # Push interval (default: 10s)
```

## Architecture

### High-Level Design
```
Pull Mode (default):
Locust Web UI (:8089) → locust_exporter (:9646) → Prometheus → Grafana
                            /stats/requests

Push Mode (optional):
Locust Web UI (:8089) → locust_exporter (:9646) → Pushgateway (:9091) → Prometheus → Grafana
                            /stats/requests          (periodic push)
```

The exporter supports both **pull-based** (default) and **push-based** (optional) operation:

**Pull Mode (Default):**
1. Receives scrape requests from Prometheus
2. Fetches JSON from Locust's `/stats/requests` endpoint
3. Transforms JSON to Prometheus metrics format
4. Returns metrics to Prometheus

**Push Mode (Optional):**
1. Periodically fetches metrics from Locust
2. Pushes metrics to Prometheus Pushgateway
3. Prometheus scrapes the Pushgateway
4. Useful for short-lived tests or CI/CD pipelines

### Core Components
1. **Exporter**: Main struct implementing `prometheus.Collector` interface
2. **HTTP Client**: Fetches data from Locust with configurable timeout
3. **Metrics Registry**: Prometheus gauges and gauge vectors
4. **HTTP Server**: Exposes `/metrics` endpoint for Prometheus

### Data Flow
```
Prometheus Scrape 
  → Exporter.Collect()
  → Exporter.scrape()
  → fetchHTTP("/stats/requests")
  → Parse JSON into locustStats struct
  → Set Gauge values (with Reset() to prevent stale data)
  → Return metrics to Prometheus
```

## Code Style Guidelines

### Project Structure
- **Single-file project**: All code in `main.go` (~456 lines)
- **Tests**: `main_test.go` (~390 lines, 80% coverage)
- **No vendor directory**: Uses Go modules
- **Go version**: 1.24

### Imports
- Use standard library grouping: stdlib → external → internal
- Currently uses: `crypto/tls`, `encoding/json`, `io`, `net/http`, `log/slog`, `github.com/prometheus/*`, `gopkg.in/alecthomas/kingpin.v2`

### Formatting & Naming
- **Standard Go formatting**: Use `gofmt` (enforced by `make format`)
- **Naming**: camelCase for unexported, PascalCase for exported. Struct fields follow Prometheus conventions (e.g., `locustUsers`, `locustRunning`)
- **Metrics naming**: Pattern is `{namespace}_{subsystem}_{name}` (e.g., `locust_requests_avg_response_time`, `locust_requests_response_time_percentile_95`)

### Types & Patterns
- Use `prometheus.Gauge` and `prometheus.GaugeVec` for all metrics (no Counters/Histograms since Locust provides snapshots)
- Implement `prometheus.Collector` interface (`Describe()`, `Collect()` methods)
- Use mutex locking (`sync.RWMutex`) when accessing metrics in `Describe()` and `Collect()`
- **Call `.Reset()` on all GaugeVec metrics** in `scrape()` to prevent stale data from old endpoints

### Error Handling
- Log errors with `slog` (Go built-in structured logging)
- Return errors from constructors (e.g., `NewExporter() (*Exporter, error)`)
- Fatal errors use `slog.Error()` + `os.Exit(1)` in `main()`
- HTTP errors: Check status code range `200-299`, return error otherwise

## Design Patterns

1. **Exporter Pattern**: Standard Prometheus exporter implementing Collector interface
2. **Factory Function**: `NewExporter()` handles initialization
3. **Functional Options**: `fetchHTTP()` returns a closure with configured client
4. **Mutex Locking**: Protects concurrent metric access during scraping

## Dependencies

### Core Dependencies
```
github.com/prometheus/client_golang v1.23.2  # Prometheus client library
github.com/prometheus/common v0.67.4         # Prometheus utilities
gopkg.in/alecthomas/kingpin.v2 v2.2.6       # CLI flag parsing
```

### Dependency Philosophy
- **Minimal Dependencies**: Only 3 direct dependencies
- **Standard Library First**: Use Go stdlib where possible (e.g., `log/slog`, `io`)
- **Prometheus Compatibility**: Must work with Prometheus scraping

## Special Considerations

### Security
- **TLS verification is DISABLED** (`InsecureSkipVerify: true`) - may be intentional for internal load testing environments
- No active CVEs (as of Dec 2025)

### Docker
- **Base image**: Uses `golang:1.24-bookworm` (Debian-based) NOT Alpine
- **Reason**: ARM64 builds fail with Alpine busybox triggers in QEMU emulation
- **Multi-platform support**: amd64, arm64

### API Compatibility
- Locust API changed JSON structure for percentiles: now uses nested `current_response_time_percentiles` object
- Must handle both global and per-endpoint percentiles (95th, 99th)
- **Important**: Always call `.Reset()` on GaugeVec metrics to prevent stale endpoint data

### Known Issues
- **No retry logic**: Single attempt to fetch from Locust, fails immediately
- **Excludes self**: Filters out "/stats/requests" endpoint from metrics
- Test mock data must match current Locust API format (nested percentiles)

## CI/CD Pipeline

Located in `.github/workflows/`:
- `ci.yml` - Test, lint, build, Docker build + Trivy scan
- `release.yml` - Multi-platform binaries + Docker with security gate
- `trivy-aqua.yml` - Weekly published image monitoring
- `release-drafter.yml` - Auto-generate release notes

All GitHub Actions pinned to commit SHAs for security.

## Resources

- **Upstream**: https://github.com/ContainerSolutions/locust_exporter
- **Fork**: https://github.com/rvankruistum/locust-exporter
- **DockerHub**: https://hub.docker.com/r/rvankruistum/locust-exporter
- **Grafana Dashboard**: https://grafana.com/grafana/dashboards/11985
