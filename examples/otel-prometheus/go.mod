module example-otel-prometheus

go 1.25

require (
	github.com/agilira/balios v0.0.0
	github.com/agilira/balios/otel v0.0.0
	github.com/prometheus/client_golang v1.20.4
	go.opentelemetry.io/otel/exporters/prometheus v0.53.0
	go.opentelemetry.io/otel/sdk/metric v1.31.0
)

require (
	github.com/agilira/argus v1.0.6 // indirect
	github.com/agilira/flash-flags v1.1.5 // indirect
	github.com/agilira/go-errors v1.1.1 // indirect
	github.com/agilira/go-timecache v1.0.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/mattn/go-sqlite3 v1.14.32 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.60.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	go.opentelemetry.io/otel v1.31.0 // indirect
	go.opentelemetry.io/otel/metric v1.31.0 // indirect
	go.opentelemetry.io/otel/sdk v1.31.0 // indirect
	go.opentelemetry.io/otel/trace v1.31.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
	google.golang.org/protobuf v1.35.1 // indirect
)

replace github.com/agilira/balios => ../../

replace github.com/agilira/balios/otel => ../../otel
