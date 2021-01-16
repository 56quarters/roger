# Roger

[![build status](https://circleci.com/gh/56quarters/roger.svg?style=shield)](https://circleci.com/gh/56quarters/roger)

Prometheus exporter for dnsmasq (DNS and DHCP daemon) and networking metrics.

## Features

* Scrape DNS related metrics from a local `dnsmasq` server
* Scrape networking related metrics from the local machine

## Building

To build from source you'll need Go 1.15 installed.

```
git clone git@github.com:56quarters/roger.git && cd roger
make build
```

The `roger` binary will then be in the root of the checkout.

## Install

At the moment, Roger is GNU/Linux specific. As such, these instructions assume a
GNU/Linux system.

To install Roger after building as described above:

* Copy the binary to `/usr/local/bin`

```
sudo cp roger /usr/local/bin/
```

* Copy and enable the Systemd unit

```
sudo cp ext/roger.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable roger.service
```

* Start the daemon

```
sudo systemctl start roger.service
```

## Usage

Roger is meant to run on a machine acting as a DNS and DHCP server for your
network (something running `dnsmasq`). It defaults to collecting metrics from
a locally running `dnsmasq` server.

Prometheus metrics are exposed on port `9779` at `/metrics` by default. Once it
is running at the host as a Prometheus target under the `scrape_configs` section
as described by the example below.

```yaml
# Sample config for Prometheus.

global:
  scrape_interval:     15s
  evaluation_interval: 15s
  external_labels:
      monitor: 'my_prom'

scrape_configs:
  - job_name: roger
    static_configs:
      - targets: ['example:9779']
```

For more information about customizing how Roger is run, see `./roger --help`.

## Development

To build a binary:

```
make build
```

To build a tagged release binary:

```
make build-dist
```

To build a Docker image

```
make image
```

To build a tagged release Docker image

```
make image-dist
```

To run tests

```
make test
```

To run lints

```
make lint
```

## License

Licensed under either of
* Apache License, Version 2.0 ([LICENSE-APACHE](LICENSE-APACHE) or http://www.apache.org/licenses/LICENSE-2.0)
* MIT license ([LICENSE-MIT](LICENSE-MIT) or http://opensource.org/licenses/MIT)

at your option.
