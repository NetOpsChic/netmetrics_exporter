package arista

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"netmetrics_exporter/internal/inventory"
	"netmetrics_exporter/internal/metrics"
)

type AristaCollector struct{}

func (c AristaCollector) Collect(device inventory.Device) error {
	// Try to enable eAPI via SSH (non-fatal)
	if err := ensureEAPIEnabled(device); err != nil {
		fmt.Printf("⚠️  Skipping eAPI enable for %s (%s): %v\n", device.Hostname, device.IP, err)
	}

	// Fetch metrics via JSON-RPC
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

	// 1) Interfaces
	if len(result) > 0 {
		ifaces, _ := result[0]["interfaceStatuses"].(map[string]interface{})
		for name, raw := range ifaces {
			d := raw.(map[string]interface{})

			up := 0.0
			if s, _ := d["lineProtocolStatus"].(string); s == "connected" {
				up = 1
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

	// 2) BGP peers
	if len(result) > 1 {
		if vrfs, ok := result[1]["vrfs"].(map[string]interface{}); ok {
			if def, ok := vrfs["default"].(map[string]interface{}); ok {
				if peers, ok := def["peers"].(map[string]interface{}); ok {
					metrics.BGPPeers.WithLabelValues(device.Hostname, device.Vendor).Set(float64(len(peers)))
				}
			}
		}
	} else {
		metrics.BGPPeers.WithLabelValues(device.Hostname, device.Vendor).Set(0)
	}

	// 3) Version & uptime
	if len(result) > 2 {
		verBlock := result[2]
		if uptime, ok := verBlock["uptime"].(float64); ok {
			metrics.DeviceUptimeSeconds.WithLabelValues(device.Hostname).Set(uptime)
		}
		if model, ok := verBlock["modelName"].(string); ok {
			if ver, ok := verBlock["version"].(string); ok {
				metrics.DeviceInfo.WithLabelValues(device.Hostname, model, ver).Set(1)
			}
		}
	}

	// 4) OSPF neighbors
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

	// 5) Interface error counters
	if len(result) > 4 {
		if counters, ok := result[4]["interfaceCounters"].(map[string]interface{}); ok {
			for iface, raw := range counters {
				d := raw.(map[string]interface{})
				inE, _ := d["inputErrors"].(float64)
				outE, _ := d["outputErrors"].(float64)
				metrics.InterfaceInputErrors.WithLabelValues(device.Hostname, iface).Set(inE)
				metrics.InterfaceOutputErrors.WithLabelValues(device.Hostname, iface).Set(outE)
			}
		}
	}

	// 6) LLDP neighbors
	if len(result) > 5 {
		if lldp, ok := result[5]["lldpNeighbors"].([]interface{}); ok {
			metrics.LLDPNeighbors.WithLabelValues(device.Hostname, device.Vendor).Set(float64(len(lldp)))
		} else {
			metrics.LLDPNeighbors.WithLabelValues(device.Hostname, device.Vendor).Set(-1)
		}
	}

	return nil
}

// ensureEAPIEnabled attempts to SSH into the switch (agent → password → keyboard‑interactive)
// and run the CLI commands to enable HTTP/HTTPS eAPI.
func ensureEAPIEnabled(device inventory.Device) error {
	// Build a list of auth methods
	auth := []ssh.AuthMethod{ssh.Password(device.Password)}

	// SSH agent support
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			auth = append(auth, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
		}
	}

	// Keyboard‑interactive fallback:
	auth = append(auth, ssh.KeyboardInteractive(
		func(user, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range questions {
				answers[i] = device.Password
			}
			return answers, nil
		},
	))

	sshConfig := &ssh.ClientConfig{
		User:            device.Username,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", device.IP), sshConfig)
	if err != nil {
		return fmt.Errorf("SSH connect failed: %w", err)
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return fmt.Errorf("SSH session failed: %w", err)
	}
	defer session.Close()

	commands := []string{
		"enable",
		"configure terminal",
		"management api http-commands",
		"  protocol http",
		"  protocol https",
		"  no shutdown",
		"end",
		"write memory",
	}
	cmd := strings.Join(commands, "\n") + "\n"

	// Run all commands in one go:
	out, err := session.CombinedOutput(cmd)
	if err != nil {
		return fmt.Errorf("failed to enable eAPI via CLI: %w\n%s", err, out)
	}

	fmt.Printf("✅ eAPI enabled via SSH for %s (%s)\n", device.Hostname, device.IP)
	return nil
}

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
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
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
