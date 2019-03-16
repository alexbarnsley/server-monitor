package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/resty.v1"
)

func runServerChecks(server *ServerConfig) {
	server.inProgress = true

	checks := make(map[string]Check, 0)
	// checks := server.Checks
	for _, group := range groups {
		for _, serverGroup := range server.Groups {
			if group.Name == serverGroup {
				for _, check := range group.Checks {
					checks[check.Name] = check
				}
				// checks = append(checks, group.Checks...)
			}
		}
	}
	for _, check := range server.Checks {
		checks[check.Name] = check
	}
	for _, check := range checks {
		response, err := server.Session.RunCommand(check.Command, true)
		var postCheck func()
		checkResult := &ServerCheck{
			Server: server,
			Check:  &check,
			Passed: true,
		}
		if err != nil {
			checkResult.Passed = false
			postCheck = func() {
				go SendAlerts(checkResult, nil, fmt.Sprintf("%s (%s)", server.Name, check.Name), fmt.Sprintf("Failed to run check '%s': %s", check.Name, err.Error()))
			}
			// Error("1 ", err)
		} else if check.ResponseContains != "" {
			if !strings.Contains(response.String(), check.ResponseContains) {
				checkResult.Passed = false
				postCheck = func() {
					go SendAlerts(checkResult, nil, fmt.Sprintf("%s (%s)", server.Name, check.Name), fmt.Sprintf("'%s' failed with response: %s", check.Name, response.String()))
				}
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
						checkResult.Passed = false
					}
					if check.Regex.LessThan != nil && intVal >= *check.Regex.LessThan {
						errors = append(errors, fmt.Sprintf("'%v' is greater than '%v': %v", check.Name, *check.Regex.LessThan, actualResult))
						checkResult.Passed = false
					}
					if check.Regex.Equals != "" && resultEntry[*check.Regex.Index] != check.Regex.Equals {
						errors = append(errors, fmt.Sprintf("'%v' does not equal '%v': %v", check.Name, check.Regex.Equals, actualResult))
						checkResult.Passed = false
					}
				}
				if !checkResult.Passed {
					uniqueErrors := make(map[string]bool, 0)
					finalErrors := make([]string, 0)
					for _, errorMessage := range errors {
						if uniqueErrors[errorMessage] != true {
							uniqueErrors[errorMessage] = true
							finalErrors = append(finalErrors, errorMessage)
						}
					}
					postCheck = func() {
						go SendAlerts(checkResult, nil, fmt.Sprintf("%s (%s)", server.Name, check.Name), strings.Join(finalErrors, ", "))
					}
				}
			}
		}
		if postCheck != nil {
			postCheck()
		}
		if checkResult.Passed {
			Info(server.Name, " - '", check.Name, "' check passed")
		} else {
			if server.CanIntervene(checkResult.Check) {
				go server.Intervene(checkResult)
			}
			Error(server.Name, " - '", check.Name, "' check failed")
		}
		err = checkResult.Save()
		if err != nil {
			Error("Could not save server result: ", err)
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

	checkResult := &WebsiteCheck{
		Website: website,
		Passed:  true,
	}
	errors := make([]string, 0)
	fail := func(text string, parts ...interface{}) {
		checkResult.Passed = false
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

	if checkResult.Passed {
		Info("Website test '", website.Name, "' passed")
	} else {
		go SendAlerts(nil, checkResult, fmt.Sprintf("%s failed", website.Name), strings.Join(errors, ", "))
		Error("Website test '", website.Name, "' failed with the following errors: ")
		for _, err := range errors {
			Error("  - ", err)
		}
	}
	err := checkResult.Save()
	if err != nil {
		Error("Could not save server result: ", err)
	}

	website.inProgress = false
}

func main() {
	CheckConfigChanges()
	InitiateDatabase()
	go StartApi()
	for {
		if HasConfigChanges() {
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
			CheckConfigChanges()
		}
		for i := 0; i < len(servers); i++ {
			server := &servers[i]
			if !server.Enabled {
				continue
			}

			if server.Session == nil {
				SendAlerts(&ServerCheck{
					Server: server,
					Passed: false,
				}, nil, "Server not connected", "Server not connected, cannot run checks")
				continue
			}

			if !server.inProgress {
				go runServerChecks(server)
			}
		}
		for i := 0; i < len(websites); i++ {
			website := &websites[i]

			if !website.Enabled {
				continue
			}

			if !website.inProgress {
				go runWebsiteChecks(website)
			}
		}
		time.Sleep(config.CheckInterval * time.Second)
	}

	return
}
