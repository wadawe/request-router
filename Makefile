.PHONY: all
all: clean build test

.PHONY: local
local: clean build_local test

WORKSPACE:=$(shell pwd)

.PHONY: clean
clean:
	rm -rf ${WORKSPACE}/bin

.PHONY: build
build:
	mkdir -p ${WORKSPACE}/bin
	docker build -f Dockerfile -t request-router-builder .
	docker run --name request-router-builder-container -v '${WORKSPACE}/bin:/go/src/github.com/wadawe/request-router/bin' request-router-builder
	docker rm -f request-router-builder-container || true

.PHONY: build_local
build_local:
	go run build.go build

.PHONY: test
test: 
	./bin/request-router -dry-run -config ./template.conf -log-dir ./logs
	./bin/request-router -dry-run -config ./examples/influxdb_v1.conf -log-dir ./logs
