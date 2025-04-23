package inventory

import (
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v2"
)

type AnsibleYAML struct {
	All struct {
		Hosts    map[string]map[string]interface{} `yaml:"hosts"`
		Vars     map[string]interface{}            `yaml:"vars"`
		Children map[string]struct {
			Vars  map[string]interface{}            `yaml:"vars"`
			Hosts map[string]map[string]interface{} `yaml:"hosts"`
		} `yaml:"children"`
	} `yaml:"all"`
}

func LoadAnsibleYAML(path string) []Device {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("Failed to read Ansible inventory file: %v", err)
	}

	var ansibleInv AnsibleYAML
	if err := yaml.Unmarshal(data, &ansibleInv); err != nil {
		log.Fatalf("Failed to parse Ansible YAML: %v", err)
	}

	var devices []Device

	// Parse all.hosts
	for hostname, vars := range ansibleInv.All.Hosts {
		devices = append(devices, buildDevice(hostname, vars, ansibleInv.All.Vars))
	}

	// Parse all.children
	for _, group := range ansibleInv.All.Children {
		for hostname, hostVars := range group.Hosts {
			merged := mergeVars(group.Vars, hostVars)
			devices = append(devices, buildDevice(hostname, merged, ansibleInv.All.Vars))
		}
	}

	return devices
}

func getString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
func buildDevice(hostname string, vars map[string]interface{}, global map[string]interface{}) Device {
	// Precedence: host > group > global
	all := mergeVars(global, vars)

	networkOS := getString(all["ansible_network_os"])
	vendor := NormalizeVendor(networkOS)

	protocol := "unknown"
	switch vendor {
	case "arista":
		protocol = "eapi"
	case "srlinux":
		protocol = "jsonrpc"
	case "cisco":
		protocol = "restconf"
	}

	return Device{
		Hostname: hostname,
		IP:       getString(all["ansible_host"]),
		Username: getString(all["ansible_user"]),
		Password: getString(all["ansible_password"]),
		Vendor:   vendor,
		Protocol: protocol,
	}
}

func mergeVars(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

var vendorMap = map[string]string{
	"eos":                   "arista",
	"ios":                   "cisco",
	"junos":                 "juniper",
	"nokia.srlinux.srlinux": "srlinux",
}

func NormalizeVendor(os string) string {
	if v, ok := vendorMap[os]; ok {
		return v
	}
	return os
}
