# Agent Guidelines for locust_exporter

## Build/Test/Lint Commands
- **Build**: `make build` or `go build -o locust_exporter .`
- **Test**: `make test` (runs `go test ./...`) - Note: Currently NO tests exist
- **Lint**: `make lint` (uses golangci-lint v1.36.0)
- **Format**: `make format` or `gofmt -w .`
- **Vet**: `make vet` or `go vet ./...`
- **Run locally**: `go run main.go --locust.uri=http://localhost:8089`

## Code Style Guidelines

### Imports
- Use standard library grouping: stdlib → external → internal (though this is a single-file project)
- Currently uses: `crypto/tls`, `encoding/json`, `io`, `net/http`, `github.com/prometheus/*`, `gopkg.in/alecthomas/kingpin.v2`

### Formatting & Naming
- **Standard Go formatting**: Use `gofmt` (enforced by `make style`)
- **Naming**: camelCase for unexported, PascalCase for exported. Struct fields follow Prometheus conventions (e.g., `locustUsers`, `locustRunning`)
- **Metrics naming**: Pattern is `{namespace}_{subsystem}_{name}` (e.g., `locust_requests_avg_response_time`)

### Types & Patterns
- Use `prometheus.Gauge` and `prometheus.GaugeVec` for all metrics (no Counters/Histograms since Locust provides snapshots)
- Implement `prometheus.Collector` interface (`Describe()`, `Collect()` methods)
- Use mutex locking (`sync.RWMutex`) when accessing metrics in `Describe()` and `Collect()`

### Error Handling
- Log errors with `log.Errorf()` from `prometheus/common/log` (deprecated - should migrate to `go-kit/log`)
- Return errors from constructors (e.g., `NewExporter() (*Exporter, error)`)
- Fatal errors use `log.Fatal()` in `main()`
- HTTP errors: Check status code range `200-299`, return error otherwise

## Special Considerations
- **Single-file project**: All code in `main.go` (~456 lines)
- **No vendor directory**: Uses Go modules
- **Security note**: TLS verification is DISABLED (`InsecureSkipVerify: true`) - may be intentional for internal use
- **Prometheus exporter pattern**: Standard pattern used by official Prometheus exporters
