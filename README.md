# Request Router

[![License][license-img]][license-href]

[license-img]: https://img.shields.io/badge/license-MIT-blue.svg
[license-href]: ./LICENSE
[config-href]: ./example.conf
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

* **Advanced Routing** : Route HTTP-based requests to different endpoints based on configurable filters and strategies.
* **Dynamic Configuration** : Easily update backend configurations without restarting the service.
* **Request Filtering** : Apply filters to route requests based on headers or query parameters.
* **Request Replication** : Forward requests to other destinations, without affecting responses to the client.
* **Request Logging** : Configurable logging for different components and levels.
* **Header Setting** : Configure custom headers for requests to backend instances.

## Limitations

* **No Buffering Support** : Other services have included some sort of in-memory buffer, but this solution does not implement any retry or queuing logic; failed requests will be dropped immediately.

## Terminology

* **Connection** : An individual upstream endpoint capable of receiving HTTP requests. It represents a single, addressable server or service instance.
* **Service** : A logical grouping of one or more connections that form a service. Requests are routed to a service when selected by a target.
* **Backend** : The internal subsystem that manages all services and their associated connections at runtime. It provides an interface for the router to dispatch requests and supports dynamic reloading.
* **Router** : A component that listens for incoming HTTP requests and evaluates them against defined routing rules. It uses the backend to resolve services and forward requests accordingly.
* **Path** : A specific route within a router that matches incoming requests based on defined criteria. Each path delegates matching requests to one or more targets.
* **Target** : A routing rule within a path that determines how and where to forward a request. Targets can filter requests, apply routing logic, and optionally replicate requests elsewhere.
* **Replica** : A secondary destination that receives a copy of the request when specified by a target. Responses from replicas are ignored by the router.
* **Filter** : A conditional rule used to determine whether a target should handle a request. Filters extract values from a specific source and compare them to regular expressions.
* **Request Strategy** : The algorithm used to decide how a request is routed to one or more connections within a service.
* **Request Action** : The behavior the router should take when a target is selected.
* **Context** : An internal data structure used to track information about a request as it moves through the router. It carries timestamps, trace metadata, status codes, and arbitrary key-value pairs.

## Requirements

- Docker
- GNU Make
- [Go](https://golang.org/doc/install) `>=1.21.3`

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

A client can make relevant HTTP requests to a router which is running and bound to a specific port via the router manager. The router then tests the request against all path endpoints and methods. If the request matches a path endpoint and method, the path then assess all of its targets to determine which service the request is for. The request is evaluated by each of the target's filters, using the defined filter strategy, to determine if the request matches the requirements for being routed to the target's destination service. If it matches, the request is sent to that service using the target's request strategy. If it does not match, the next target is assessed, and so on. If no match is found, the request is rejected.

### Configuration

A full break-down of the configuration file structure can be found in the [example.conf][config-href] file.

### Development

Below is a directory structure of the [`pkg/`][pkg-href] folder, which contains the source code for the service, along with a brief description of each directory and its purpose:

```
pkg/                        # Source code for the service.
├── backend/                # Manages connections to different backend services.
├── config/                 # Contains configuration schemas, structures, validation, and loading logic.
├── service/                # Contains logic for running the router service(s).
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
