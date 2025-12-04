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
