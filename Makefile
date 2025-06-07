export GO ?= go

.DEFAULT_GOAL := build

.PHONY: docker-build
docker-build:
	docker build -t http-tls-proxy .

.PHONY: docker-run
docker-run:
	docker run -d --restart always --net internal --publish 3128:3128 --name http-tls-proxy http-tls-proxy

.PHONY: run
run:
	${GO} run ./cmd/

.PHONY: build
build:
	${GO} build -o http-tls-proxy ./cmd/

.PHONY: clean
clean:
	${RM} http-tls-proxy

.PHONY: lint
lint:
	golangci-lint run
