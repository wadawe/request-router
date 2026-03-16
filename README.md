# Request Router

[![License][license-img]][license-href]

[license-img]: https://img.shields.io/badge/license-MIT-blue.svg
[license-href]: ./LICENSE
[config-href]: ./template.conf
[pkg-href]: ./pkg/

---

## Overview

**Request Router** is a lightweight, high-performance HTTP router and reverse proxy designed for flexible request routing, load balancing, and configuration-driven behavior.

This project began as a purpose-built rewrite of `influxdb-relay`, originally developed to support internal metrics infrastructure at the BBC. While the initial implementation focused on efficiently proxying InfluxDB write and query requests, this reworked version decouples the core routing logic from any Influx-specific behavior. It has been redesigned as a more general-purpose solution for a wide range of HTTP routing use cases.

This version draws inspiration and lessons from earlier community efforts, including:
- https://github.com/bbc/relay-for-influxdb (original rewrite by @wadawe)
- https://github.com/toni-moreno/influxdb-srelay
- https://github.com/veepee-oss/influxdb-relay
- https://github.com/influxdata/influxdb-relay

The goal of `request-router` is to offer a streamlined, configuration-first routing engine that's easy to deploy, extensible, and performant under load. It was designed to address stability issues found in other implementations, while adding modern features like dynamic config reloads and flexible route matching.

> Written by [@wadawe](https://github.com/wadawe)

## Features

* **Flexible Request Routing** : Route HTTP requests to multiple backends using configurable paths, targets, and routing strategies.
* **Dynamic Configuration Reloading** : Update connections and services on the fly without downtime.
* **Target Filtering** : Apply rules to selectively route requests based on headers or query parameters.
* **Request Replication** : Optionally forward requests to secondary replica services for auditing or redundancy.
* **Customisable Logging** : Configure access and target logs for easy tracability.
* **Support for HTTP/1.1 and HTTP/2** : Choose your preferred HTTP version per router, with TLS support.

## Limitations

* **No Buffering Support** : Other services have included some sort of in-memory buffer, but this solution does not implement any retry or queuing logic; failed requests will be dropped immediately.

## Terminology

* **Connection**: A single backend HTTP endpoint capable of handling requests. Connections are addressable server instances that the router forwards requests to on behalf of a registered service.
* **Service**: A logical grouping of connections. Targets route requests to services, which handle them according to the selected request strategy.
* **Router**: A listener bound to a specific address and port. It matches incoming requests to configured paths and routes them accordingly.
* **Path**: A routing definition within a router that matches on URL and HTTP method. Each path delegates request handling to one or more targets.
* **Target**: A rule within a path that defines how a matching request should be handled. Targets support filtering, routing logic, and optional replication to secondary destinations (replicas).
* **Replica**: A secondary service that receives a forwarded copy of a request from a target. Replica responses are not returned to the client.
* **Request Action**: The action a target takes when selected. Can be `forward`, `reject`, `simulate`, or `offload`.
* **Request Strategy**: The method used to select connections for routing within a target. Includes `ping`, `primary`, `sequence`, `success`, and `highest`.
* **Filter Strategy**: Determines how filters are evaluated within a target. Can be `all` (all filters must match) or `any` (at least one must match).
* **Request Filter**: A condition that checks if a request matches based on headers or query parameters, using regular expressions.
* **Header Override**: A key/value header setting applied to requests before forwarding them to the target service. Can also be used to remove headers by setting their value to an empty string.
* **Core**: The central orchestration layer responsible for coordinating and managing the lifecycle of all routers, including starting, stopping, and monitoring them.
* **Backend**: The internal subsystem responsible for managing services and their corresponding connections. It supports runtime reloading and request dispatching.
* **Context**: A per-request data structure that tracks metadata such as trace steps, status codes, and logging details as the request flows through a router.
* **Access Log**: A per-router log that records summary information for each processed request, including trace path, status codes, and timings.
* **Target Logger**: A per-target log used to capture detailed errors, request failures, and replica issues.
* **Admin Manager**: A built-in component that exposes HTTP endpoints for managing and monitoring the running service.
* **Endpoint**: A general term referring to any addressable URL path where a router and associated upstream connection expects to receive requests.
* **Incoming Endpoint**: The URL path where a router path expects to receive requests from clients.
* **Upstream Endpoint**: The URL path used by a target when forwarding requests to a backend service.

## Requirements

- Docker
- GNU Make
- [Go](https://golang.org/doc/install) `>=1.24`

_Other versions will probably work but are untested!_

## Build

Before building a new version, you should update the CHANGELOG! Follow the existing [SemVer](https://semver.org/) format!

After installing docker, you can run:
* `make local` to create a build locally.
    * This should create a build compatible with your current system.
* `make all` to create a build for a specific architecture & OS.
    * You may need to update the `FROM` in the [`Dockerfile`](/Dockerfile) based on your desired architecture & OS combination.

```sh
git clone https://github.com/wadawe/request-router
cd request-router
make all
```

## Usage

Incoming HTTP requests are received by a router instance bound to a specific address. Each request is matched against configured path endpoints and HTTP methods. If a matching path is found, its targets are evaluated in order.

Each target applies its filter strategy (`any` or `all`) to determine if the request meets its matching criteria, using values extracted from headers or query parameters. If matched, the target handles the request using its configured action (`forward`, `reject`, `simulate`, or `offload`). For forwarding actions, the target selects a backend service and routes the request using a defined strategy (`ping`, `primary`, `sequence`, `success`, or `highest`). 

If no target matches, the request is rejected with a `400 Bad Request`.

### Configuration

A full break-down of the configuration file structure can be found in the [template.conf][config-href] file.

### Development

Below is a directory structure of the [`pkg/`][pkg-href] folder, which contains the source code for the service, along with a brief description of each directory and its purpose:

```
pkg/                        # Source code for the service.
├── backend/                # Manages connections to different backend services.
├── config/                 # Contains configuration schemas, structures, validation, and loading logic.
├── core/                   # Contains logic for running the router service(s).
│   ├── context/            # Manages context for individual requests to the router.
│   └── router/             # Handles routing of requests. 
├── utils/                  # Contains utility functions.
└── main.go                 # Main entry point for the service.
```

### Command Line Flags

There are a series of command-line flags that can be used when running the service.

| Flag    | Example                        | Required? | Description |
|---------|--------------------------------|-----------|-------------|
| config  | `-config=/path/to/config.conf` | Y         | Define a path to the service config. |
| log-dir | `-log-dir=/path/to/logs/`      | N         | Define a directory to place service log files. If not defined, the working directory of the process is used. |
| pidfile | `-pidfile=/path/to/pid`        | N         | Define a path to the service pidfile. |
| agent   | `-agent=request-router`        | N         | Define a custom User-Agent value to use for router requests (headers). |
| dry-run | `-dry-run`                     | N         | Validates config without starting the service. |
| version | `-version`                     | N         | Print the service version and exit. |
| help    | `-help`                        | N         | Print the service help message and exit. |

### Logging

Each router can be configured to write to an access log. This file outputs a line per request, displaying information such as the response `status code`, a `trace` path for the request, request and response lengths, and more.

Each target can also be configured to write to a separate log file. This is where relevant errors are output for requests to the target service, if any. It is advised to monitor these error files as they can be useful to determine erroneous requests, malformed data, etc.

### Signals

The service handles several Unix signals to control its behavior. Below is a list of supported signals and their corresponding actions:

#### `SIGTERM` | `SIGQUIT` | `SIGINT`

The service will stop gracefully, and then exit the process.

For example; termination (15), quit (3), or interrupt (2) signal can be sent to the router if it's run via a systemd service with:

```
systemctl kill --signal=SIGTERM request-router
systemctl kill --signal=SIGQUIT request-router
systemctl kill --signal=SIGINT request-router
```

#### `SIGHUP`

The service will reload its backend configuration, and log the result of the reload operation.

For example; a hangup (1) signal can be sent to the router if it's run via a systemd service with:

```
systemctl kill --signal=SIGHUP request-router
```

_Please note that `[[connection]]` and `[[service]]` sections only will be re-read and updated by the backend. `[[router]]` definition updates require a service restart._

#### Other...

Other signals can be sent to the service, but will be ignored and no actions will be taken.

---

## Contributing

Contributions are welcome! 

Whether it's bug fixes, feature suggestions, or improvements to documentation; we appreciate community involvement. Feel free to submit issues & pull requests.
