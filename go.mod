module github.com/gustycube/spyder-probe

go 1.22

require (
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/go-redis/redis/v9 v9.5.1
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/prometheus/client_golang v1.19.1
	github.com/temoto/robotstxt v1.1.2
	go.uber.org/zap v1.27.0
	golang.org/x/net v0.30.0
	golang.org/x/time v0.5.0
)

require (
	go.opentelemetry.io/otel v1.28.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.28.0
	go.opentelemetry.io/otel/sdk v1.28.0
)
