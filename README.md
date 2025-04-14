
# netmetrics_exporter

`netmetrics_exporter` is a Go-based Network device metric exporter designed for collecting network metrics from routers. Inspired by `node_exporter`, this exporter provides deep observability for network engineers and NetDevOps workflows.

> ✅ Currently supports **Arista EOS** via eAPI over HTTP.

---

## 🔧 Features

- Interface status (up/down)
- Interface speed (bandwidth)
- Duplex mode
- BGP neighbor count
- OSPF neighbor count
- Interface error counters (input/output)
- LLDP neighbor count
- Device info (model, version, uptime)

---

## 📦 Installation

```bash
git clone https://github.com/netopschic/netmetrics_exporter.git
cd netmetrics_exporter
make build
```

---

## 🚀 Usage

```bash
./bin/netmetrics_exporter --inventory ansible-inventory.yaml --listen-address :9200
```

- `--inventory` → Path to Ansible-compatible inventory file.
- `--listen-address` → Address to expose Prometheus metrics (default `:9200`).

---

## 📘 Sample Inventory (this has to be a running Arista router)

```yaml
all:
  hosts:
    R1:
      ansible_host: 192.168.100.xx
      ansible_user: admin
      ansible_password: admin
      ansible_network_os: eos
```

---

## 🔍 Example Output

Visit: [http://localhost:9200/metrics](http://localhost:9200/metrics)

```
# HELP netmetrics_interface_up Interface status (1=up, 0=down)
# TYPE netmetrics_interface_up gauge
netmetrics_interface_up{device="R1",interface="Ethernet1",vendor="arista"} 1

# HELP netmetrics_bgp_peers Total BGP peers
# TYPE netmetrics_bgp_peers gauge
netmetrics_bgp_peers{device="R1",vendor="arista"} 2
```

---

## 🛣 Roadmap

- [x] Arista EOS support
- [ ] Cisco IOS & Junos (via NAPALM)
- [ ] Native Prometheus service discovery integration
- [ ] Containerized release for easy deployment

---

## 🤝 Contributing

Pull requests are welcome. Let’s build the soul of NetDevOps observability together.

---

## 🧠 About

Built with ❤️ by [netopschic](https://github.com/netopschic) to make NetDevOps monitoring just as powerful as system observability.