# Locust Exporter

> **Note:** This is a maintained fork of [ContainerSolutions/locust_exporter](https://github.com/ContainerSolutions/locust_exporter), which is no longer actively maintained. This fork includes security updates, Go 1.24, comprehensive tests, and modern CI/CD practices.

Prometheus exporter for [Locust](https://github.com/locustio/locust). This exporter was inspired by [mbolek/locust_exporter](https://github.com/mbolek/locust_exporter).

[![Docker Pulls](https://img.shields.io/docker/pulls/rvankruistum/locust-exporter.svg)](https://hub.docker.com/r/rvankruistum/locust-exporter/tags) 
[![License](https://img.shields.io/github/license/rvankruistum/locust-exporter.svg)](https://github.com/rvankruistum/locust-exporter/blob/main/LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/rvankruistum/locust-exporter)](https://github.com/rvankruistum/locust-exporter)
[![Tests](https://github.com/rvankruistum/locust-exporter/workflows/CI/badge.svg)](https://github.com/rvankruistum/locust-exporter/actions)

![locust_dashboard](locust_dashboard.png)

## What's New in This Fork

This maintained fork includes significant improvements:

- **Modern Go**: Updated from Go 1.12 to Go 1.24
- **Security**: All CVEs fixed, dependencies updated to latest stable versions
- **Testing**: Comprehensive test suite with 80% code coverage
- **CI/CD**: Modern GitHub Actions workflows with security scanning (Trivy, CodeQL)
- **Build System**: Simplified Makefile, removed Promu dependency
- **Docker**: Multi-platform support (amd64, arm64), optimized multi-stage builds
- **Supply Chain Security**: All GitHub Actions pinned to commit SHAs, Dependabot enabled
- **Logging**: Migrated from deprecated prometheus/common/log to log/slog

See [CHANGELOG.md](CHANGELOG.md) for detailed changes.

## Quick Start

This package is available for Docker:

1. Run Locust ([example docker-compose](https://github.com/locustio/locust/blob/master/examples/docker-compose/docker-compose.yml))

2. Run Locust Exporter

    with docker:

    ```bash
    docker run --net=host rvankruistum/locust-exporter
    ```

    or with docker-compose:

    ```yaml
    version: "3.0"

    services:
      locust-exporter:
        image: rvankruistum/locust-exporter
        network_mode: "host"
    ```

3. Modify `prometheus.yml` to add target to Prometheus

    ```yaml
    scrape_configs:
      - job_name: 'locust'
        static_configs:
          - targets: ['<LOCUST_IP>:9646']
    ```

4. Add dashboard to Grafana with ID [11985](https://grafana.com/grafana/dashboards/11985)

## Building and Running

### Prerequisites

- Go 1.24 or later
- Docker (optional, for containerized builds)

### Quick Build

```bash
# Clone the repository
git clone https://github.com/rvankruistum/locust-exporter.git
cd locust-exporter

# Build using Makefile
make build

# Run
./locust_exporter --locust.uri=http://localhost:8089
```

### Flags

- `--locust.uri`
  Address of Locust. Default is `http://localhost:8089`.

- `--locust.timeout`
  Timeout request to Locust. Default is `5s`.

- `--web.listen-address`
  Address to listen on for web interface and telemetry. Default is `:9646`.

- `--web.telemetry-path`
  Path under which to expose metrics. Default is `/metrics`.

- `--locust.namespace`
  Namespace for prometheus metrics. Default `locust`.

- `--pushgateway.url`
  URL of Prometheus Pushgateway. If empty, push mode is disabled. Default is empty (disabled).

- `--pushgateway.job`
  Job label for Pushgateway metrics. Default is `locust`.

- `--pushgateway.interval`
  Interval for pushing metrics to Pushgateway. Default is `10s`.

- `--log.level`
  Set logging level: one of `debug`, `info`, `warn`, `error`, `fatal`

- `--log.format`
  Set the log output target and format. e.g. `logger:syslog?appname=bob&local=7` or `logger:stdout?json=true`
  Defaults to `logger:stderr`.

### Environment Variables

The following environment variables configure the exporter:

- `LOCUST_EXPORTER_URI`
  Address of Locust. Default is `http://localhost:8089`.

- `LOCUST_EXPORTER_TIMEOUT`
  Timeout reqeust to Locust. Default is `5s`.

- `LOCUST_EXPORTER_WEB_LISTEN_ADDRESS`
  Address to listen on for web interface and telemetry. Default is `:9646`.

- `LOCUST_EXPORTER_WEB_TELEMETRY_PATH`
  Path under which to expose metrics. Default is `/metrics`.

- `LOCUST_METRIC_NAMESPACE`
  Namespace for prometheus metrics. Default `locust`.

- `LOCUST_EXPORTER_PUSHGATEWAY_URL`
  URL of Prometheus Pushgateway. If empty, push mode is disabled. Default is empty (disabled).

- `LOCUST_EXPORTER_PUSHGATEWAY_JOB`
  Job label for Pushgateway metrics. Default is `locust`.

- `LOCUST_EXPORTER_PUSHGATEWAY_INTERVAL`
  Interval for pushing metrics to Pushgateway. Default is `10s`.

## Push Mode (Pushgateway Support)

For short-lived Locust tests, you can push metrics to a [Prometheus Pushgateway](https://github.com/prometheus/pushgateway) instead of relying on Prometheus scraping.

> **⚠️ Warning:** Push mode requires Prometheus Pushgateway to be deployed and accessible. 

### When to Use Push Mode

Push mode is particularly useful in these scenarios:

- **Local Development**: Running Locust locally while using a containerized exporter, where Prometheus can't easily scrape your local setup
- **Clusters Without Prometheus**: Running load tests in environments where Prometheus is not deployed or accessible
- **CI/CD Pipelines**: Batch load tests that run and complete quickly in CI/CD workflows
- **Short-Lived Tests**: Tests that don't run long enough for Prometheus to scrape at its configured interval

**Note:** If you have Prometheus already set up and scraping your infrastructure, the default pull-based mode is recommended. Push mode adds the Pushgateway as an intermediary component.

### Basic Usage

```bash
./locust_exporter \
  --locust.uri=http://localhost:8089 \
  --pushgateway.url=http://pushgateway:9091
```

### How It Works

1. Locust Exporter fetches metrics from Locust every scrape interval
2. If Pushgateway is configured, metrics are pushed at the specified interval (default 10s)
3. Prometheus scrapes the Pushgateway to retrieve the metrics
4. The `/metrics` endpoint remains available for direct scraping (hybrid mode)

### Notes

- Push mode is **completely optional** - if not configured, the exporter works exactly as before
- Both pull (scraping) and push modes can run simultaneously
- Metrics are pushed using the `push` method, which replaces all previous metrics for the job
- When a Locust test ends, metrics naturally go to zero (users, RPS, etc.)

### Grafana

The grafana dashboard has beed published with ID [11985](https://grafana.com/grafana/dashboards/11985) and was exported to [locust_dashboard.json](locust_dashboard.json).

### Screenshot

[![locust exporter](locust_exporter.png)](locust_exporter.md)

### Changelog

Please see [CHANGELOG](CHANGELOG.md) for more information on what has changed recently.

## Contributing

Please see [CONTRIBUTING](CONTRIBUTING.md) for details.

## License

Apache License 2.0. Please see [LICENSE](LICENSE) for more information.

## Original Project

This is a fork of [ContainerSolutions/locust_exporter](https://github.com/ContainerSolutions/locust_exporter).  
Original copyright 2019 Container Solutions.  
See [NOTICE](NOTICE) for attribution details.
