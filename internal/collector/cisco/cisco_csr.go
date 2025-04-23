package cisco

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"netmetrics_exporter/internal/inventory"
	"netmetrics_exporter/internal/metrics"
)

type CollectorCSR struct{}

func (c CollectorCSR) Collect(device inventory.Device) error {
	baseURL := fmt.Sprintf("https://%s/restconf/data", device.IP)
	client := &http.Client{}
	headers := map[string]string{
		"Accept": "application/yang-data+json",
	}

	// ===== Interface Metrics =====
	intfURL := baseURL + "/ietf-interfaces:interfaces"
	intfBody, err := restconfGet(client, intfURL, device.Username, device.Password, headers)
	if err == nil {
		var interfaceData struct {
			Interfaces struct {
				Interface []struct {
					Name    string `json:"name"`
					Enabled bool   `json:"enabled"`
					IPv4    struct {
						Address []struct {
							IP string `json:"ip"`
						} `json:"address"`
					} `json:"ietf-ip:ipv4"`
				} `json:"interface"`
			} `json:"ietf-interfaces:interfaces"`
		}
		if err := json.Unmarshal(intfBody, &interfaceData); err == nil {
			for _, intf := range interfaceData.Interfaces.Interface {
				up := 0.0
				if intf.Enabled {
					up = 1.0
				}
				metrics.InterfaceUp.WithLabelValues(device.Hostname, intf.Name, device.Vendor).Set(up)
			}
		}
	}

	// ===== BGP Metrics =====
	bgpURL := baseURL + "/Cisco-IOS-XE-bgp-oper:bgp-state-data"
	bgpBody, err := restconfGet(client, bgpURL, device.Username, device.Password, headers)
	if err == nil {
		var bgpData struct {
			Neighbors []interface{} `json:"neighbors"`
		}
		if err := json.Unmarshal(bgpBody, &bgpData); err == nil {
			metrics.BGPPeers.WithLabelValues(device.Hostname, device.Vendor).Set(float64(len(bgpData.Neighbors)))
		}
	}

	// ===== LLDP Neighbors =====
	lldpURL := baseURL + "/Cisco-IOS-XE-lldp-oper:lldp-entries"
	lldpBody, err := restconfGet(client, lldpURL, device.Username, device.Password, headers)
	if err == nil {
		var lldpData struct {
			Entries []struct {
				LocalInterface string `json:"local-interface"`
			} `json:"lldp-entry"`
		}
		if err := json.Unmarshal(lldpBody, &lldpData); err == nil {
			metrics.LLDPNeighbors.WithLabelValues(device.Hostname, device.Vendor).Set(float64(len(lldpData.Entries)))
		}
	}

	// ===== CPU Usage =====
	cpuURL := baseURL + "/Cisco-IOS-XE-process-cpu-oper:cpu-usage"
	cpuBody, err := restconfGet(client, cpuURL, device.Username, device.Password, headers)
	if err == nil {
		var cpuData struct {
			FiveSec int `json:"five-seconds"`
		}
		if err := json.Unmarshal(cpuBody, &cpuData); err == nil {
			metrics.CPUUsage.WithLabelValues(device.Hostname, device.Vendor).Set(float64(cpuData.FiveSec))
		}
	}

	// ===== Memory Usage =====
	memURL := baseURL + "/Cisco-IOS-XE-memory-oper:memory-statistics"
	memBody, err := restconfGet(client, memURL, device.Username, device.Password, headers)
	if err == nil {
		var memData struct {
			MemoryStats []struct {
				Used  int `json:"used-memory"`
				Total int `json:"total-memory"`
			} `json:"memory-statistic"`
		}
		if err := json.Unmarshal(memBody, &memData); err == nil && len(memData.MemoryStats) > 0 {
			usedPct := float64(memData.MemoryStats[0].Used) / float64(memData.MemoryStats[0].Total) * 100
			metrics.MemoryUsage.WithLabelValues(device.Hostname, device.Vendor).Set(usedPct)
		}
	}
	// ===== OSPF Neighbors =====
	ospfURL := baseURL + "/Cisco-IOS-XE-ospf-oper:ospf-oper-data"
	ospfBody, err := restconfGet(client, ospfURL, device.Username, device.Password, headers)
	if err == nil {
		var ospfData struct {
			Ospfv2 []struct {
				Ospfv2Neighbor []interface{} `json:"ospfv2-neighbor"`
			} `json:"ospf-state/neighbors"`
		}
		if err := json.Unmarshal(ospfBody, &ospfData); err == nil {
			count := 0
			for _, ospf := range ospfData.Ospfv2 {
				count += len(ospf.Ospfv2Neighbor)
			}
			metrics.OSPFNeighbors.WithLabelValues(device.Hostname, device.Vendor).Set(float64(count))
		}
	}

	return nil
}

func restconfGet(client *http.Client, url, username, password string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(username, password)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig.InsecureSkipVerify = true
	client.Transport = transport

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("RESTCONF error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("RESTCONF error: %s - %s", resp.Status, string(body))
	}

	return ioutil.ReadAll(resp.Body)
}
