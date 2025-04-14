package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	arista "netmetrics_exporter/internal/collector/arista"
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

	// Polling loop
	go func() {
		for {
			for _, dev := range devices {
				switch dev.Vendor {
				case "arista":
					err := arista.AristaCollector{}.Collect(dev)
					if err != nil {
						log.Printf("[ERROR] %s: %v", dev.Hostname, err)
					}
				default:
					log.Printf("[WARN] unsupported vendor: %s", dev.Vendor)
				}
			}
			time.Sleep(30 * time.Second)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
