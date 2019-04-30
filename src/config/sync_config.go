package config

type SyncConfig struct {
	Interval int `yaml:"interval"`
	Timeout int `yaml:"timeout"`
	Failure MonitorConfig  `yaml:"failure"`
	Recover MonitorConfig  `yaml:"recover"`
	URL string `yaml:"url"`
	CheckCode bool `yaml:"check_code"`
}

type MonitorConfig struct {
	Count int `yaml:"count"`
}

func NewDefaultSync() *SyncConfig {
	return &SyncConfig{
		Interval: 2,
		Timeout: 3,
		Failure: MonitorConfig{
			Count: 3,
		},
		Recover: MonitorConfig{
			Count: 2,
		},
		URL: "/health",
		CheckCode: false,
	}
}
