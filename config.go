package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"gopkg.in/resty.v1"
)

type Check struct {
	Name             string
	SeverityType     string `json:"severity"`
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

type ElasticConfig struct {
	Host     string
	Port     int
	Username string
	Password string
}

type MonitorConfig struct {
	CheckInterval time.Duration
	Alerts        AlertConfig
	Elastic       ElasticConfig
}

type SeverityConfig struct {
	CheckMinutes             time.Duration
	FailedAttemptsPercentage int16
	AlertResendMinutes       time.Duration
	Alerts                   map[string]bool
}

type ServerConfig struct {
	Name         string
	Host         string
	Port         int16
	Username     string
	Password     string
	SeverityType string `json:"severity"`
	Groups       []string
	Checks       []Check
	Session      *sshSession
	Alerts       map[string]bool
	inProgress   bool
}

type WebsiteConfig struct {
	Name              string
	SeverityType      string `json:"severity"`
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
	loadDefault    bool
	loadMethod     func(string) error
	preLoadMethod  func()
	postLoadMethod func()
}

var configFileOrder []string
var configFiles map[string]configFile
var config MonitorConfig
var severity map[string]SeverityConfig
var servers []ServerConfig
var groups []GroupConfig
var websites []WebsiteConfig

var httpClient = resty.New()

func init() {
	configFileOrder = []string{
		"global",
		"severity",
		"servers",
		"groups",
		"websites",
	}
	configFiles = map[string]configFile{
		"global": {
			path:       "config.json",
			loadMethod: loadMonitorConfig,
		},
		"severity": {
			path:        "severity.json",
			loadMethod:  loadSeverityConfig,
			loadDefault: true,
		},
		"servers": {
			path:           "servers.json",
			loadMethod:     loadServerConfig,
			preLoadMethod:  disconnectAllServers,
			postLoadMethod: connectToServers,
			loadDefault:    true,
		},
		"groups": {
			path:           "groups.json",
			loadMethod:     loadGroupsConfig,
			preLoadMethod:  disconnectAllServers,
			postLoadMethod: connectToServers,
			loadDefault:    true,
		},
		"websites": {
			path:           "websites.json",
			loadMethod:     loadWebsitesConfig,
			preLoadMethod:  disconnectAllServers,
			postLoadMethod: connectToServers,
			loadDefault:    true,
		},
	}

	httpClient.SetHTTPMode()
}

func loadJson(configName string) []byte {
	config := configFiles[configName]
	path, pathError := config.getFilePath()
	if pathError != nil {
		Fatal("Could not find config `", configName, "`: ", pathError)
	}
	configJson, readError := ioutil.ReadFile(path)
	if readError != nil {
		Fatal("Could not load ", path, " config: ", readError)
	}

	return configJson
}

func loadMonitorConfig(configName string) error {
	return json.Unmarshal(loadJson(configName), &config)
}

func loadSeverityConfig(configName string) error {
	return json.Unmarshal(loadJson(configName), &severity)
}

func loadServerConfig(configName string) error {
	err := json.Unmarshal(loadJson(configName), &servers)
	if err != nil {
		return err
	}

	for _, server := range servers {
		if server.SeverityType == "" {
			Warn("Severity not specified for server `", server.Name, "`")
		} else if _, ok := severity[server.SeverityType]; !ok {
			Warn("Severity `", server.SeverityType, "` does not exist for server `", server.Name, "`")
		}
	}

	return nil
}

func loadGroupsConfig(configName string) error {
	return json.Unmarshal(loadJson(configName), &groups)
}

func loadWebsitesConfig(configName string) error {
	// websites = make([]WebsiteConfig, 0)
	err := json.Unmarshal(loadJson(configName), &websites)
	if err != nil {
		return err
	}

	for _, website := range websites {
		if website.SeverityType == "" {
			Warn("Severity not specified for website check `", website.Name, "`")
		} else if _, ok := severity[website.SeverityType]; !ok {
			Warn("Severity `", website.SeverityType, "` does not exist for website check `", website.Name, "`")
		}
	}

	return nil
}

func updateConfigModifiedTime(name string) {
	thisConfig := configFiles[name]
	modifiedTime, err := thisConfig.getConfigModifiedTime()
	if err != nil {
		Error("Could not update modified time for `", name, "`: ", err)

		return
	}
	thisConfig.modifiedTime = modifiedTime
	configFiles[name] = thisConfig
}

func CheckConfigChanges() {
	// for configName, config := range configFiles {
	for _, configName := range configFileOrder {
		config, ok := configFiles[configName]
		if !ok {
			Error("Could not load options for config file `", configName, "`")
			continue
		}

		if hasChanged, _ := config.hasConfigChanged(config.modifiedTime); hasChanged {
			InfoBold("Loading ", configName, " config file...")
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

func HasConfigChanges() bool {
	for _, config := range configFiles {
		if hasChanged, _ := config.hasConfigChanged(config.modifiedTime); hasChanged {
			return true
		}
	}

	return false
}

func (config *configFile) getConfigModifiedTime() (time.Time, error) {
	path, err := config.getFilePath()
	if err != nil {
		return time.Time{}, err
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		Fatal("Could not find config file `", path, "`")
	}

	return fileInfo.ModTime(), nil
}

func (config *configFile) hasConfigChanged(modifiedTime time.Time) (bool, error) {
	configModifiedTime, err := config.getConfigModifiedTime()
	if err != nil {
		return true, errors.New(fmt.Sprintf("Could not get config modified time: %v", err))
	}

	return !modifiedTime.Equal(configModifiedTime), nil
}

func (config *configFile) getFilePath() (string, error) {
	path := "config/" + config.path
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = "config/default/" + config.path
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return "", errors.New(fmt.Sprintf("Config file `%v` does not exist", config.path))
		}
	}

	return path, nil
}

func (server *ServerConfig) CanSendAlert(alert string, defaultValue bool) bool {
	if val, ok := server.Alerts[alert]; ok {
		return val
	}

	return defaultValue
}

func getSeverity(severityType string) *SeverityConfig {
	if severityType == "" {
		return nil
	}

	if severityConfig, ok := severity[severityType]; ok {
		return &severityConfig
	}

	return nil
}

func (check *Check) Severity() *SeverityConfig {
	return getSeverity(check.SeverityType)
}

func (server *ServerConfig) Severity() *SeverityConfig {
	return getSeverity(server.SeverityType)
}

func (website *WebsiteConfig) Severity() *SeverityConfig {
	return getSeverity(website.SeverityType)
}
