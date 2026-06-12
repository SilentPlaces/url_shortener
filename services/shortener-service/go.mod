module github.com/arminaray/url_shortener/services/shortener-service

go 1.25.3

require (
	github.com/arminaray/url_shortener/pkg/httpx v0.0.0-00010101000000-000000000000
	github.com/arminaray/url_shortener/pkg/safeurl v0.0.0-00010101000000-000000000000
	github.com/lib/pq v1.10.9
	github.com/prometheus/client_golang v1.20.5
	github.com/redis/go-redis/v9 v9.17.0
	github.com/segmentio/kafka-go v0.4.48
	golang.org/x/sync v0.10.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	go.opentelemetry.io/otel v1.32.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.32.0 // indirect
	go.opentelemetry.io/otel/metric v1.32.0 // indirect
	go.opentelemetry.io/otel/sdk v1.32.0 // indirect
	go.opentelemetry.io/otel/trace v1.32.0 // indirect
	golang.org/x/sys v0.27.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
)

replace (
	github.com/arminaray/url_shortener/pkg/httpx => ../../pkg/httpx
	github.com/arminaray/url_shortener/pkg/safeurl => ../../pkg/safeurl
)
