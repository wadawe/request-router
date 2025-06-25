# v0.0.1

Initial release of **Request Router**, a general-purpose HTTP router and reverse proxy.

This release is a review and rewrite of [`bbc/relay-for-influxdb`](https://github.com/bbc/relay-for-influxdb), refactored to support flexible routing, filtering, and backend service management across a wide range of HTTP-based use cases.

- Added support for routing HTTP requests to different services based on path endpoints and HTTP methods.
- Added support for evaluating requests using configurable target filters.
- Added support for filter strategies: `any` and `all`.
- Added support for filter sources: headers and query parameters.
- Added support for per-target upstream path overrides.
- Added request actions: `forward`, `reject`, `simulate`, and `offload`.
- Added request strategies: `ping`, `primary`, `sequence`, `success`, and `highest`.
- Added request replication to replica services without affecting client responses.
- Added support for dynamic backend config reloads via `SIGHUP`.
- Added TOML-based configuration for defining routers, paths, targets, services, and connections.
- Added support for structured logging with configurable log levels.
- Added per-target logging to separate files.
- Added request/response body handling helpers for safe reading and closing.
- Added internal request context tracking with trace metadata and status codes.
- Added backend layer for managing services and their connections at runtime.
- Added documentation including usage, configuration, and development guidance.

> Original codebase: [bbc/relay-for-influxdb](https://github.com/bbc/relay-for-influxdb)
