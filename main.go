package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/resty.v1"
)

func connectToServers() {
	for i := 0; i < len(servers); i++ {
		server := &servers[i]
		session, err := sshConnect(server)
		if err != nil {
			Error("Failed to connect to '", server.Name, "': ", err.Error())
		}
		servers[i].Session = session
	}
}

func disconnectAllServers() {
	if len(servers) > 0 {
		for i := 0; i < len(servers); i++ {
			err := servers[i].Session.client.Close()
			if err != nil {
				Error("Could not close SSH session for '", servers[i].Name, "': ", err.Error())
			}
		}
	}
}

func CanSendAlert(server *ServerConfig, alert string, defaultValue bool) bool {
	if val, ok := server.Alerts[alert]; ok {
		return val
	}

	return defaultValue
}

func SendAlerts(server *ServerConfig, subject string, message string) {
	Error("Alert - ", message)
	if config.Alerts.SimplePush.Enabled && CanSendAlert(server, "simplePush", config.Alerts.SimplePush.Default) {
		// AlertSimplePush(subject, message)
	}
	// if config.Alerts.Email.Enabled && CanSendAlert(server, "email", config.Alerts.Email.Default) {
	// 	AlertEmail(subject, message)
	// }
	// if config.Alerts.SMS.Enabled && CanSendAlert(server, "sms", config.Alerts.SMS.Default) {
	// 	AlertSMS(subject, message)
	// }
}

func runServerChecks(server *ServerConfig) {
	server.inProgress = true

	checks := server.Checks
	for _, group := range groups {
		for _, serverGroup := range server.Groups {
			if group.Name == serverGroup {
				checks = append(checks, group.Checks...)
			}
		}
	}
	for j := 0; j < len(checks); j++ {
		passed := true
		check := checks[j]
		response, err := server.Session.RunCommand(check.Command)
		if err != nil {
			go SendAlerts(server, fmt.Sprintf("Alert - %s (%s)", server.Name, check.Name), fmt.Sprintf("Failed to run check '%s': %s", check.Name, err.Error()))
			passed = false
			continue
		}
		if check.ResponseContains != "" {
			if !strings.Contains(response.String(), check.ResponseContains) {
				go SendAlerts(server, fmt.Sprintf("Alert - %s (%s)", server.Name, check.Name), fmt.Sprintf("'%s' failed with response: %s", check.Name, response.String()))
				passed = false
				continue
			}
		} else if check.Regex != nil && check.Regex.Expression != "" {
			if check.Regex.Index == nil {
				Warn("Index for regex not provided for check '", check.Name, "'")
			} else {
				regex, _ := regexp.Compile(check.Regex.Expression)
				result := regex.FindAllStringSubmatch(response.String(), -1)
				errors := make([]string, 0)
				for _, resultEntry := range result {
					actualResult := resultEntry[*check.Regex.Index]
					intVal, _ := strconv.Atoi(actualResult)
					if check.Regex.GreaterThan != nil && intVal <= *check.Regex.GreaterThan {
						errors = append(errors, fmt.Sprintf("'%v' is less than '%v': %v", check.Name, *check.Regex.GreaterThan, actualResult))
						passed = false
					}
					if check.Regex.LessThan != nil && intVal >= *check.Regex.LessThan {
						errors = append(errors, fmt.Sprintf("'%v' is greater than '%v': %v", check.Name, *check.Regex.LessThan, actualResult))
						passed = false
					}
					if check.Regex.Equals != "" && resultEntry[*check.Regex.Index] != check.Regex.Equals {
						errors = append(errors, fmt.Sprintf("'%v' does not equal '%v': %v", check.Name, check.Regex.Equals, actualResult))
						passed = false
					}
				}
				if !passed {
					uniqueErrors := make(map[string]bool, 0)
					finalErrors := make([]string, 0)
					for _, errorMessage := range errors {
						if uniqueErrors[errorMessage] != true {
							uniqueErrors[errorMessage] = true
							finalErrors = append(finalErrors, errorMessage)
						}
					}
					SendAlerts(server, fmt.Sprintf("Alert - %s (%s)", server.Name, check.Name), strings.Join(finalErrors, ", "))
				}
			}
		}
		if passed {
			Info(server.Name, " - '", check.Name, "' check passed")
		}
	}

	server.inProgress = false
}

func runWebsiteChecks(website *WebsiteConfig) {
	website.inProgress = true

	var response *resty.Response
	var responseError error
	if website.Method == "" || website.Method == "GET" {
		response, responseError = httpClient.R().Get(website.Url)
	} else if website.Method == "POST" {
		request := httpClient.R()
		for header, value := range website.RequestHeaders {
			request.SetHeader(header, value)
		}
		request.SetBody(website.RequestBody)
		response, responseError = request.Post(website.Url)
	}

	errors := make([]string, 0)
	fail := func(text string, parts ...interface{}) {
		errors = append(errors, fmt.Sprintf(text, parts...))
	}

	if responseError != nil {
		fail("Failed request: ", responseError.Error())
	} else {
		if website.StatusCode != 0 && website.StatusCode != response.StatusCode() {
			fail("Status code - expected '%v', got '%v'", website.StatusCode, response.Status())
		}
		if website.MaxResponseTimeMS != 0 {
			responseTimeMS := response.Time().Seconds() * 1000
			Debug(responseTimeMS, website.MaxResponseTimeMS)
			if responseTimeMS > website.MaxResponseTimeMS {
				fail("Response time - expected below '%v' ms, took '%v' ms", website.MaxResponseTimeMS, responseTimeMS)
			}
		}
		if len(website.ResponseHeaders) > 0 {
			for header, headerCheckValue := range website.ResponseHeaders {
				if responseHeader, ok := website.ResponseHeaders[header]; ok {
					if headerCheckValue == "" {
						fail("Header '%v' should not exist", header)
					} else if responseHeader != headerCheckValue {
						fail("Header '%v' - expected '%v', got '%v'", header, headerCheckValue, responseHeader)
					}
				}
			}
		}
	}

	if len(errors) == 0 {
		Info("Website test '", website.Name, "' passed")
	} else {
		Error("Website test '", website.Name, "' failed with the following errors: ")
		for _, err := range errors {
			Error("  - ", err)
		}
	}

	website.inProgress = false
}

func main() {
	for {
		if hasConfigChanges() {
			for {
				hasRunning := false
				for i := 0; i < len(servers); i++ {
					if servers[i].inProgress {
						hasRunning = true
						break
					}
				}
				if hasRunning {
					continue
				}
				for i := 0; i < len(websites); i++ {
					if websites[i].inProgress {
						hasRunning = true
						break
					}
				}
				if !hasRunning {
					break
				}
			}
			checkConfigChanges()
		}
		for i := 0; i < len(servers); i++ {
			server := &servers[i]
			if server.Session == nil {
				SendAlerts(server, "Server not connected", "Server not connected, cannot run checks")
				continue
			}

			if !server.inProgress {
				go runServerChecks(server)
			}
		}
		for i := 0; i < len(websites); i++ {
			website := &websites[i]
			if !website.inProgress {
				go runWebsiteChecks(website)
			}
		}
		time.Sleep(config.CheckInterval * time.Second)
	}

	return
}
