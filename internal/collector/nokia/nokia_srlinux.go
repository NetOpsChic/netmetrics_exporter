package nokia

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"netmetrics_exporter/internal/inventory"
	"netmetrics_exporter/internal/metrics"
)

type SRLinuxCollector struct{}

func (c SRLinuxCollector) Collect(device inventory.Device) error {
	if err := ensureHTTPAPIEnabled(device); err != nil {
		return fmt.Errorf("JSON-RPC not enabled: %v", err)
	}

	// === 1. System Info ===
	sysResp, err := runRPC(device, []string{"/system/information"})
	if err != nil {
		return fmt.Errorf("Failed to fetch system info: %v", err)
	}

	version := safeStr(sysResp["version"])
	description := safeStr(sysResp["description"])
	metrics.DeviceInfo.WithLabelValues(device.Hostname, description, version).Set(1)

	if bootedAt, ok := sysResp["last-booted"].(string); ok {
		parsed, err := time.Parse(time.RFC3339, bootedAt)
		if err == nil {
			uptime := time.Since(parsed).Seconds()
			metrics.DeviceUptimeSeconds.WithLabelValues(device.Hostname).Set(uptime)
		} else {
			metrics.DeviceUptimeSeconds.WithLabelValues(device.Hostname).Set(-1)
		}
	}

	// === 2. Interface Info ===
	ifResp, err := runRPC(device, []string{"/interface"})
	if err == nil {
		ifList := extractNamespaceField(ifResp, "interface")
		for _, raw := range ifList {
			intf := raw.(map[string]interface{})
			name := safeStr(intf["name"])
			admin := safeStr(intf["admin-state"])
			oper := safeStr(intf["oper-state"])

			duplex := "unknown"
			if d, ok := intf["duplex-mode"]; ok {
				duplex = safeStr(d)
			} else if eth, ok := intf["ethernet"].(map[string]interface{}); ok {
				duplex = safeStr(eth["duplex-mode"])
			}

			speedMbps := float64(0)
			if eth, ok := intf["ethernet"].(map[string]interface{}); ok {
				if speed, ok := eth["port-speed"].(string); ok {
					switch speed {
					case "speed-1G":
						speedMbps = 1000
					case "speed-10G":
						speedMbps = 10000
					case "speed-100G":
						speedMbps = 100000
					}
				}
			}

			up := 0.0
			if admin == "enable" && oper == "up" {
				up = 1.0
			}

			metrics.InterfaceUp.WithLabelValues(device.Hostname, name, device.Vendor).Set(up)
			metrics.InterfaceSpeedMbps.WithLabelValues(device.Hostname, name, device.Vendor).Set(speedMbps)

			if duplex != "unknown" {
				metrics.InterfaceDuplex.WithLabelValues(device.Hostname, name, device.Vendor, duplex).Set(1)
			}
		}
	}

	// === 3. Interface Errors ===
	errResp, err := runRPC(device, []string{"/interface/statistics"})
	if err == nil {
		ifStats := extractNamespaceField(errResp, "statistics")
		for _, raw := range ifStats {
			stat := raw.(map[string]interface{})
			name := safeStr(stat["interface"])
			inErrors := toFloat(stat["in-errors"])
			outErrors := toFloat(stat["out-errors"])
			metrics.InterfaceInputErrors.WithLabelValues(device.Hostname, name).Set(inErrors)
			metrics.InterfaceOutputErrors.WithLabelValues(device.Hostname, name).Set(outErrors)
		}
	}

	// === 4. BGP Peers ===
	bgpResp, err := runRPC(device, []string{"/network-instance[name=default]/protocols/bgp/neighbor"})
	peerCount := 0.0
	if err == nil {
		if neighbors, ok := bgpResp["neighbor"].([]interface{}); ok {
			peerCount = float64(len(neighbors))
		}
	}
	metrics.BGPPeers.WithLabelValues(device.Hostname, device.Vendor).Set(peerCount)

	// === 5. OSPF Neighbors ===
	ospfNeighbors := 0.0
	ospfResp, err := runRPC(device, []string{"/network-instance[name=default]/protocols/ospf/instance[name=ospf-default]/area"})
	if err == nil {
		if areas, ok := ospfResp["area"].([]interface{}); ok {
			for _, a := range areas {
				area := a.(map[string]interface{})
				if ifs, ok := area["interface"].([]interface{}); ok {
					for _, intf := range ifs {
						if neighbors, ok := intf.(map[string]interface{})["neighbor"].([]interface{}); ok {
							ospfNeighbors += float64(len(neighbors))
						}
					}
				}
			}
		}
	}
	metrics.OSPFNeighbors.WithLabelValues(device.Hostname, device.Vendor).Set(ospfNeighbors)

	// === 6. LLDP Neighbors ===
	lldpResp, err := runRPC(device, []string{"/system/lldp/interface"})
	count := 0.0
	if err == nil {
		ifaces, ok := lldpResp["interface"].([]interface{})
		if ok {
			for _, raw := range ifaces {
				iface := raw.(map[string]interface{})
				if _, hasNeighbor := iface["neighbor"]; hasNeighbor {
					count += 1
				}
			}
		}
	}
	metrics.LLDPNeighbors.WithLabelValues(device.Hostname, device.Vendor).Set(count)

	return nil
}

// === Helpers ===

func ensureHTTPAPIEnabled(device inventory.Device) error {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "patch",
		"params": map[string]interface{}{
			"updates": []map[string]interface{}{
				{
					"path": "/system/management/interface/json-rpc",
					"value": map[string]interface{}{
						"admin-state": "enable",
					},
				},
			},
		},
		"id": 1,
	}
	data, _ := json.Marshal(payload)

	url := fmt.Sprintf("http://%s/jsonrpc", device.IP)
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
		return fmt.Errorf("failed to enable JSON-RPC: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response while enabling HTTP API: %s", string(body))
	}
	return nil
}

func runRPC(device inventory.Device, paths []string) (map[string]interface{}, error) {
	var commands []map[string]interface{}
	for _, path := range paths {
		commands = append(commands, map[string]interface{}{
			"path":      path,
			"datastore": "state",
			"recursive": true,
		})
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "get",
		"params": map[string]interface{}{
			"commands": commands,
		},
		"id": 1,
	}
	data, _ := json.Marshal(payload)

	url := fmt.Sprintf("http://%s/jsonrpc", device.IP)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(data))
	req.SetBasicAuth(device.Username, device.Password)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	debugLog("📥 SRL Raw Body from %s: %s\n", device.Hostname, string(body))

	var jsonResp map[string]interface{}
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		return nil, err
	}

	results, ok := jsonResp["result"].([]interface{})
	if !ok || len(results) == 0 {
		return nil, fmt.Errorf("no result returned")
	}

	return results[0].(map[string]interface{}), nil
}

func debugLog(format string, args ...interface{}) {
	if os.Getenv("NETMETRICS_DEBUG") == "1" {
		fmt.Printf(format, args...)
	}
}

func safeStr(v interface{}) string {
	if str, ok := v.(string); ok {
		return str
	}
	return "unknown"
}

func toFloat(v interface{}) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return -1
}

func extractNamespaceField(resp map[string]interface{}, field string) []interface{} {
	if val, ok := resp[field]; ok {
		if items, ok := val.([]interface{}); ok {
			return items
		}
	}
	for k, v := range resp {
		if bytes.Contains([]byte(k), []byte(":")) && bytes.HasSuffix([]byte(k), []byte(field)) {
			if items, ok := v.([]interface{}); ok {
				return items
			}
		}
	}
	return nil
}
