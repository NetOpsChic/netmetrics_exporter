package arista

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"netmetrics_exporter/internal/inventory"
	"netmetrics_exporter/internal/metrics"
)

type AristaCollector struct{}

func (c AristaCollector) Collect(device inventory.Device) error {
	result, err := runEAPI(device, []string{
		"show interfaces status",
		"show ip bgp summary",
		"show version",
		"show ip ospf neighbor",
		"show interfaces counters errors",
		"show lldp neighbors",
	})
	if err != nil {
		return err
	}

	if os.Getenv("NETMETRICS_DEBUG") == "1" {
		fmt.Printf("DEBUG JSON RESULT from %s: %+v\n", device.Hostname, result)
	}

	if len(result) > 0 {
		ifaces, ok := result[0]["interfaceStatuses"].(map[string]interface{})
		if ok {
			for name, raw := range ifaces {
				d := raw.(map[string]interface{})

				up := 0.0
				if s, ok := d["lineProtocolStatus"].(string); ok && s == "connected" {
					up = 1.0
				}
				metrics.InterfaceUp.WithLabelValues(device.Hostname, name, device.Vendor).Set(up)

				if bw, ok := d["bandwidth"].(float64); ok {
					metrics.InterfaceSpeedMbps.WithLabelValues(device.Hostname, name, device.Vendor).Set(bw / 1_000_000)
				} else {
					metrics.InterfaceSpeedMbps.WithLabelValues(device.Hostname, name, device.Vendor).Set(-1)
				}

				duplex := "unknown"
				if dpx, ok := d["duplex"].(string); ok {
					duplex = dpx
				}
				metrics.InterfaceDuplex.WithLabelValues(device.Hostname, name, device.Vendor, duplex).Set(1)
			}
		}
	}

	if len(result) > 1 {
		if vrfs, ok := result[1]["vrfs"].(map[string]interface{}); ok {
			if def, ok := vrfs["default"].(map[string]interface{}); ok {
				if peers, ok := def["peers"].(map[string]interface{}); ok {
					metrics.BGPPeers.WithLabelValues(device.Hostname, device.Vendor).Set(float64(len(peers)))
				}
			}
		}
	}

	if len(result) > 2 {
		version := result[2]
		if uptime, ok := version["uptime"].(float64); ok {
			metrics.DeviceUptimeSeconds.WithLabelValues(device.Hostname).Set(uptime)
		}
		if model, ok := version["modelName"].(string); ok {
			if ver, ok := version["version"].(string); ok {
				metrics.DeviceInfo.WithLabelValues(device.Hostname, model, ver).Set(1)
			}
		}
	}

	if len(result) > 3 {
		ospfCount := -1.0
		if vrfs, ok := result[3]["vrfs"].(map[string]interface{}); ok {
			if def, ok := vrfs["default"].(map[string]interface{}); ok {
				if instList, ok := def["instList"].(map[string]interface{}); ok {
					if inst, ok := instList["1"].(map[string]interface{}); ok {
						if entries, ok := inst["ospfNeighborEntries"].([]interface{}); ok {
							ospfCount = float64(len(entries))
						}
					}
				}
			}
		}
		metrics.OSPFNeighbors.WithLabelValues(device.Hostname, device.Vendor).Set(ospfCount)
	}

	if len(result) > 4 {
		if counters, ok := result[4]["interfaceCounters"].(map[string]interface{}); ok {
			for iface, raw := range counters {
				d := raw.(map[string]interface{})
				in, _ := d["inputErrors"].(float64)
				out, _ := d["outputErrors"].(float64)
				metrics.InterfaceInputErrors.WithLabelValues(device.Hostname, iface).Set(in)
				metrics.InterfaceOutputErrors.WithLabelValues(device.Hostname, iface).Set(out)
			}
		}
	}

	if len(result) > 5 {
		if lldp, ok := result[5]["lldpNeighbors"].([]interface{}); ok {
			metrics.LLDPNeighbors.WithLabelValues(device.Hostname).Set(float64(len(lldp)))
		} else {
			metrics.LLDPNeighbors.WithLabelValues(device.Hostname).Set(-1)
		}
	}

	return nil
}

// === Helper ===

func runEAPI(device inventory.Device, commands []string) ([]map[string]interface{}, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "runCmds",
		"params": map[string]interface{}{
			"version": 1,
			"cmds":    commands,
			"format":  "json",
		},
		"id": "1",
	}

	data, _ := json.Marshal(payload)

	url := fmt.Sprintf("http://%s/command-api", device.IP)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(data))
	req.SetBasicAuth(device.Username, device.Password)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	if os.Getenv("NETMETRICS_DEBUG") == "1" {
		fmt.Printf("RAW BODY from %s:\n%s\n", device.Hostname, string(body))
	}

	var jsonResp struct {
		Result []map[string]interface{} `json:"result"`
	}

	if err := json.Unmarshal(body, &jsonResp); err != nil {
		return nil, err
	}

	return jsonResp.Result, nil
}
