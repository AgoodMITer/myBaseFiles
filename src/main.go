package main

import (
	"flag"
	"strings"
	"net/http"
	"github.com/gorilla/mux"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"gopkg.in/yaml.v2"
	"github.com/mmpei/janus/src/config"
	"github.com/mmpei/janus/src/handler"
	"github.com/mmpei/janus/src/sync"
)

type sliceFlagValue []string

func (s *sliceFlagValue) Set(val string) error {
	*s = sliceFlagValue(strings.Split(val, ","))
	return nil
}

func (s *sliceFlagValue) String() string {
	*s = sliceFlagValue(strings.Split("20202,20203", ","))
	return ""
}

func main() {
	var configPath string
	var cfg = config.Configuration{}
	flag.StringVar(&configPath, "config", "", "specify the config file with formatter yaml")
	flag.StringVar(&cfg.LogLevel, "log_level", "", "log level:debug info warn error")
	flag.StringVar(&cfg.IP, "ip", "", "ip where the server listen on")
	flag.IntVar(&cfg.Port, "port", 0, "port where the server listen on")
	flag.IntVar(&cfg.ProxyPort, "proxy_port", 0, "port where the proxy server listen on")
	flag.Parse()

	if len(configPath) == 0 {
		configPath = "conf.yaml"
	}

	readConfig(configPath, &config.ProxyConfig)

	// parse args
	parseArgs(&config.ProxyConfig, &cfg)

	// check config
	if err := config.ProxyConfig.Validate(); err != nil {
		log.Errorf("parse config error: %v ", err)
		return
	}

	log.SetLevel(logLevel(config.ProxyConfig.LogLevel))

	// endpoint monitor init
	epMonitor := sync.NewMonitorManager(config.ProxyConfig.Backends, config.ProxyConfig.BackendProxiedPort, &config.ProxyConfig.Monitor)
	// sentinel init
	sentinel := sync.NewSentinel(epMonitor)

	self := fmt.Sprintf("%s:%d", config.ProxyConfig.IP, config.ProxyConfig.Port)
	var peer string
	for _, p := range config.ProxyConfig.Cluster {
		if p == self || p == config.ProxyConfig.IP {
			continue
		}
		peer = p
	}
	syncManager := sync.NewSyncManager(self, peer, &config.ProxyConfig.Sync, sentinel)
	go syncManager.Run()

	listenAddr := fmt.Sprintf("%s:%d", config.ProxyConfig.IP, config.ProxyConfig.Port)
	router := mux.NewRouter()
	h := handler.NewHandler(syncManager)
	router.HandleFunc("/sync", h.Sync).Methods("POST")
	router.HandleFunc("/info", h.Info).Methods("GET")
	go http.ListenAndServe(listenAddr, router)

	// start proxy
	select {
	}
}

func readConfig(file string, cfg *config.Configuration) error {
	if len(file) == 0 {
		return nil
	}
	buffer, err := ioutil.ReadFile(file)
	err = yaml.Unmarshal(buffer, cfg)
	if err != nil {
		return err
	}
	return nil
}

func parseArgs(dest , src *config.Configuration) {
	if len(src.LogLevel) > 0 {
		dest.LogLevel = src.LogLevel
	}
	if len(src.IP) > 0 {
		dest.IP = src.IP
	}
	if src.Port > 0 {
		dest.Port = src.Port
	}
}

func logLevel(level string) log.Level {
	l, err := log.ParseLevel(level)
	if err != nil {
		l = log.InfoLevel
		log.Warnf("error parsing level %s: %v, using %q	", level, err, l)
	}
	return l
}
