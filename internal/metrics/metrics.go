package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	InterfaceUp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "netmetrics_interface_up",
			Help: "Indicates whether the interface is up (1) or down (0).",
		},
		[]string{"hostname", "interface", "vendor"},
	)

	BGPPeers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "netmetrics_bgp_neighbors_total",
			Help: "Total number of BGP peers per device.",
		},
		[]string{"hostname", "vendor"},
	)

	InterfaceSpeedMbps = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "netmetrics_interface_speed_mbps",
			Help: "Interface speed in Mbps",
		},
		[]string{"hostname", "interface", "vendor"},
	)
	InterfaceDuplex = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "netmetrics_interface_duplex",
			Help: "Interface duplex mode (label: duplex=duplexFull|duplexHalf|unknown)",
		},
		[]string{"hostname", "interface", "vendor", "duplex"},
	)
	DeviceUptimeSeconds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "netmetrics_device_uptime_seconds",
			Help: "Device uptime in seconds",
		},
		[]string{"hostname"},
	)

	DeviceInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "netmetrics_device_info",
			Help: "Device model and OS version",
		},
		[]string{"hostname", "model", "version"},
	)
	OSPFNeighbors = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "netmetrics_ospf_neighbors_total",
			Help: "Number of OSPF neighbors",
		},
		[]string{"hostname", "vendor"},
	)
	InterfaceInputErrors = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "netmetrics_interface_input_errors_total",
			Help: "Input errors per interface",
		},
		[]string{"hostname", "interface"},
	)

	InterfaceOutputErrors = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "netmetrics_interface_output_errors_total",
			Help: "Output errors per interface",
		},
		[]string{"hostname", "interface"},
	)

	LLDPNeighbors = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "netmetrics_lldp_neighbors_total",
			Help: "Number of LLDP neighbors",
		},
		[]string{"hostname", "vendor"},
	)

	DeviceMemoryTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "net_device_memory_total_mb",
			Help: "Total memory (in MB)",
		},
		[]string{"hostname"},
	)

	DeviceMemoryFree = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "net_device_memory_free_mb",
			Help: "Free memory (in MB)",
		},
		[]string{"hostname"},
	)
	CPUUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "netmetrics_cpu_usage_percent",
			Help: "CPU usage percent reported by the device",
		},
		[]string{"hostname", "vendor"},
	)
	MemoryUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "netmetrics_memory_usage_percent",
			Help: "Memory usage percent calculated from total/used",
		},
		[]string{"hostname", "vendor"},
	)
)

func Register() {
	prometheus.MustRegister(InterfaceUp)
	prometheus.MustRegister(BGPPeers)
	prometheus.MustRegister(InterfaceSpeedMbps)
	prometheus.MustRegister(InterfaceDuplex)
	prometheus.MustRegister(DeviceUptimeSeconds)
	prometheus.MustRegister(DeviceInfo)
	prometheus.MustRegister(OSPFNeighbors)
	prometheus.MustRegister(InterfaceInputErrors)
	prometheus.MustRegister(InterfaceOutputErrors)
	prometheus.MustRegister(LLDPNeighbors)
	prometheus.MustRegister(DeviceMemoryTotal)
	prometheus.MustRegister(DeviceMemoryFree)
	prometheus.MustRegister(CPUUsage)
	prometheus.MustRegister(MemoryUsage)
}
