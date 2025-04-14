APP=netmetrics_exporter
PKG=netmetrics_exporter

build:
	@echo "Building $(APP)..."
	go build -o bin/$(APP) -ldflags "\
	  -X '$(PKG)/internal/version.Version=0.1.0' \
	  -X '$(PKG)/internal/version.Commit=$$(git rev-parse --short HEAD)' \
	  -X '$(PKG)/internal/version.BuildDate=$$(date -u +%Y-%m-%dT%H:%M:%SZ)'" ./cmd/netmetrics_exporter

run:
	bin/$(APP) --inventory ansible-inventory.yaml --listen-address :9200

clean:
	rm -rf bin/
