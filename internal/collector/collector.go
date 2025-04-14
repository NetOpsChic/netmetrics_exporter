package collector

import "netmetrics_exporter/internal/inventory"

type Collector interface {
	Collect(device inventory.Device) error
}
