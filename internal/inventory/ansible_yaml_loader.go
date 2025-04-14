package inventory

import (
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v2"
)

type AnsibleYAML struct {
	All struct {
		Hosts map[string]map[string]interface{} `yaml:"hosts"`
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
	for hostname, vars := range ansibleInv.All.Hosts {
		dev := Device{
			Hostname: hostname,
			IP:       getString(vars["ansible_host"]),
			Username: getString(vars["ansible_user"]),
			Password: getString(vars["ansible_password"]),
			Vendor:   NormalizeVendor(getString(vars["ansible_network_os"])),
			Protocol: "eapi", // optional: default to eAPI for eos
		}
		devices = append(devices, dev)
	}

	return devices
}

func getString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

var vendorMap = map[string]string{
	"eos":   "arista",
	"ios":   "cisco",
	"junos": "juniper",
}

func NormalizeVendor(os string) string {
	if v, ok := vendorMap[os]; ok {
		return v
	}
	return os
}
