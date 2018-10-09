package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"

	"gopkg.in/resty.v1"
)

type Check struct {
	Name             string
	Command          string
	ResponseContains string
	Regex            *Regex
	Alert            string
}

type Regex struct {
	Expression  string
	Index       *int
	GreaterThan *int
	LessThan    *int
	Equals      string
	Contains    string
}

type SimplePushAlert struct {
	Code    string
	Enabled bool
	Default bool
}

type AlertConfig struct {
	SimplePush SimplePushAlert
}

type MonitorConfig struct {
	CheckInterval time.Duration
	Alerts        AlertConfig
}

type ServerConfig struct {
	Name       string
	Host       string
	Port       int16
	Username   string
	Password   string
	Groups     []string
	Checks     []Check
	Session    *sshSession
	Alerts     map[string]bool
	inProgress bool
}

type WebsiteConfig struct {
	Name              string
	Url               string
	Method            string
	StatusCode        int
	MaxResponseTimeMS float64
	ResponseHeaders   map[string]string
	RequestHeaders    map[string]string
	RequestBody       string
	inProgress        bool
}

type GroupConfig struct {
	Name   string
	Checks []Check
}

type configFile struct {
	path           string
	modifiedTime   time.Time
	loadMethod     func(string) error
	preLoadMethod  func()
	postLoadMethod func()
}

var configFiles map[string]configFile
var config MonitorConfig
var servers []ServerConfig
var groups []GroupConfig
var websites []WebsiteConfig

var httpClient = resty.New()

func init() {
	configFiles = map[string]configFile{
		"global": {
			path:       "config/config.json",
			loadMethod: loadMonitorConfig,
		},
		"servers": {
			path:           "config/servers.json",
			loadMethod:     loadServerConfig,
			preLoadMethod:  disconnectAllServers,
			postLoadMethod: connectToServers,
		},
		"groups": {
			path:           "config/groups.json",
			loadMethod:     loadGroupsConfig,
			preLoadMethod:  disconnectAllServers,
			postLoadMethod: connectToServers,
		},
		"websites": {
			path:           "config/websites.json",
			loadMethod:     loadWebsitesConfig,
			preLoadMethod:  disconnectAllServers,
			postLoadMethod: connectToServers,
		},
	}

	httpClient.SetHTTPMode()
}

func loadJson(configName string) []byte {
	configJson, err := ioutil.ReadFile(configFiles[configName].path)
	if err != nil {
		Fatal("Could not load ", configName, " config: ", err)
	}

	return configJson
}

func loadMonitorConfig(configName string) error {
	return json.Unmarshal(loadJson(configName), &config)
}

func loadServerConfig(configName string) error {
	return json.Unmarshal(loadJson(configName), &servers)
}

func loadGroupsConfig(configName string) error {
	return json.Unmarshal(loadJson(configName), &groups)
}

func loadWebsitesConfig(configName string) error {
	websites = make([]WebsiteConfig, 0)
	return json.Unmarshal(loadJson(configName), &websites)
}

func updateConfigModifiedTime(name string) {
	thisConfig := configFiles[name]
	thisConfig.modifiedTime = getConfigModifiedTime(thisConfig.path)
	configFiles[name] = thisConfig
}

func checkConfigChanges() {
	for configName, config := range configFiles {
		if hasConfigChanged(config.path, config.modifiedTime) {
			Info("Loading ", configName, " config file...")
			if config.preLoadMethod != nil {
				config.preLoadMethod()
			}
			err := config.loadMethod(configName)
			if err != nil {
				Fatal("Could not parse ", configName, " config: ", err)
			}
			updateConfigModifiedTime(configName)
			if config.postLoadMethod != nil {
				config.postLoadMethod()
			}
		}
	}
}

func hasConfigChanges() bool {
	for _, config := range configFiles {
		if hasConfigChanged(config.path, config.modifiedTime) {
			return true
		}
	}

	return false
}

func getConfigModifiedTime(path string) time.Time {
	fileInfo, err := os.Stat(path)
	if err != nil {
		Fatal("Could not find config file '", path, "'")
	}

	return fileInfo.ModTime()
}

func hasConfigChanged(path string, modifiedTime time.Time) bool {
	return !modifiedTime.Equal(getConfigModifiedTime(path))
}
