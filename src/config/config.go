package config

import "fmt"

var ProxyConfig = Configuration{
	Sync: *NewDefaultSync(),
}

type Configuration struct {
	// LogLevel
	LogLevel          string      `yaml:"log_level"`

	// Cluster the proxy cluster, now only support two endpoint
	Cluster             []string    `yaml:"cluster"`

	// IP
	IP                string  `yaml:"ip"`
	// Port
	Port              int         `yaml:"port"`
	// ProxyPort
	ProxyPort              int         `yaml:"proxy_port"`

    // Sync
	Sync SyncConfig `yaml:"sync"`

	// Monitor for backend
	Monitor SyncConfig `yaml:"monitor"`
	// backend with the control port
	Backends        []string    `yaml:"backends"`
	// the port backend listening on for server
	BackendProxiedPort int `yaml:"backend_proxied_port"`
	// ToMaster
	ToMaster                string  `yaml:"to_master"`
	// ToSlave
	ToSlave                string  `yaml:"to_slave"`
}

func (cfg *Configuration) Validate() error {
	if len(cfg.IP) == 0 {
		return fmt.Errorf("Invalid ip address ")
	}
	if len(cfg.Cluster) != 2 {
		return fmt.Errorf("Invalid cluster, now we only support 2 ep ")
	}
	if len(cfg.Backends) == 0 {
		return fmt.Errorf("Invalid backends ")
	}
	if cfg.Port == 0 {
		return fmt.Errorf("Invalid listening port ")
	}
	return nil
}
