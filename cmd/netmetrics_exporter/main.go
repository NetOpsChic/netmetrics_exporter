package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	arista "netmetrics_exporter/internal/collector/arista"
	"netmetrics_exporter/internal/collector/cisco"
	"netmetrics_exporter/internal/collector/nokia"
	"netmetrics_exporter/internal/inventory"
	"netmetrics_exporter/internal/metrics"
	"netmetrics_exporter/internal/version"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	inventoryPath := flag.String("inventory", "configs/inventory.yaml", "Path to inventory YAML file")
	listenAddress := flag.String("listen-address", ":9200", "Address to expose /metrics")
	flag.Parse()

	// Pretty banner
	fmt.Println("===================================")
	fmt.Printf("🛰️  netmetrics_exporter %s (commit %s, built at %s)\n", version.Version, version.Commit, version.BuildDate)
	fmt.Printf("📡 Listening on http://localhost%s/metrics\n", *listenAddress)
	fmt.Println("===================================")

	// Register Prometheus collectors
	metrics.Register()

	// Load inventory
	var devices []inventory.Device
	if strings.Contains(*inventoryPath, "ansible") {
		devices = inventory.LoadAnsibleYAML(*inventoryPath)
	} else {
		devices = inventory.Load(*inventoryPath)
	}

	// Debug: print loaded devices
	for _, dev := range devices {
		log.Printf("[DEBUG] Loaded Device: Hostname=%s IP=%s Vendor=%s Protocol=%s",
			dev.Hostname, dev.IP, dev.Vendor, dev.Protocol)
	}

	// Start background collection loop
	go func() {
		for {
			for _, dev := range devices {
				var err error

				switch dev.Vendor {
				case "arista":
					err = arista.AristaCollector{}.Collect(dev)

				case "srlinux":
					err = nokia.SRLinuxCollector{}.Collect(dev)

				case "cisco":
					log.Printf("[INFO] ▶️  Calling CollectorCSR for %s", dev.Hostname)
					err = cisco.CollectorCSR{}.Collect(dev)
				}

				if err != nil {
					log.Printf("[ERROR] %s (%s): %v", dev.Hostname, dev.Vendor, err)
				}
			}
			time.Sleep(30 * time.Second)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
