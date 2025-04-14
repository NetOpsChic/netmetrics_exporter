package inventory

import (
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v2"
)

type RawInventory struct {
	Devices []Device `yaml:"devices"`
}

func Load(path string) []Device {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("Error reading inventory file: %v", err)
	}

	var inv RawInventory
	if err := yaml.Unmarshal(data, &inv); err != nil {
		log.Fatalf("Error parsing inventory YAML: %v", err)
	}

	return inv.Devices
}
