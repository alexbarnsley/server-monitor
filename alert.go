package main

import (
	"fmt"
	// "github.com/davecgh/go-spew/spew"
)

func SendAlerts(serverResult *ServerCheck, websiteResult *WebsiteCheck, subject string, message string) {
	var server *ServerConfig
	var website *WebsiteConfig
	if serverResult != nil {
		server = serverResult.Server
	}
	if websiteResult != nil {
		website = websiteResult.Website
	}
	severityName := ""
	isSevere := false
	canSend := map[string]bool{
		"simplePush": false,
	}
	if serverResult != nil && server != nil {
		isSevere = serverResult != nil && serverResult.IsSevere()
		if isSevere && !serverResult.CanResendAlert() {
			return
		}
		if server.CanSendAlert("simplePush", config.Alerts.SimplePush.Default) {
			canSend["simplePush"] = true
		}
		severityName = serverResult.GetSeverityName()
	}
	if websiteResult != nil && website != nil {
		isSevere = websiteResult != nil && websiteResult.IsSevere()
		if isSevere && !websiteResult.CanResendAlert() {
			return
		}
		if website.CanSendAlert("simplePush", config.Alerts.SimplePush.Default) {
			canSend["simplePush"] = true
		}
		severityName = websiteResult.GetSeverityName()
	}
	if !isSevere {
		return
	}
	Error(fmt.Sprintf("%v ALERT - %v", severityName, subject))
	fmt.Println("canSend[canSend]", canSend["canSend"])
	if config.Alerts.SimplePush.Enabled && canSend["simplePush"] {
		AlertSimplePush(subject, message)
	}
	// if config.Alerts.SimplePush.Enabled {
	// 	if server.CanSendAlert("simplePush", config.Alerts.SimplePush.Default) {
	// 		Error("Sending simple push")
	// 		AlertSimplePush(subject, message)
	// 	}
	// }
	// if config.Alerts.Email.Enabled && CanSendAlert(server, "email", config.Alerts.Email.Default) {
	// 	AlertEmail(subject, message)
	// }
	// if config.Alerts.SMS.Enabled && CanSendAlert(server, "sms", config.Alerts.SMS.Default) {
	// 	AlertSMS(subject, message)
	// }

	var alert *Alert = &Alert{}
	if serverResult != nil {
		alert.AlertId = serverResult.GetTestId()
	}
	if websiteResult != nil {
		alert.AlertId = websiteResult.GetTestId()
	}
	if alert.AlertId == "" {
		Error("Could not save alert - no id set")
	} else {
		err := alert.Save()
		if err != nil {
			Error("Could not save alert: ", err)
		}
	}
}

func AlertSimplePush(subject string, message string) {
	response, err := httpClient.R().Get(fmt.Sprintf("https://api.simplepush.io/send/%s/%s/%s", config.Alerts.SimplePush.Code, subject, message))
	if err != nil {
		Error("Could not send SimplePush alert: ", err)
	}
	// spew.Dump(response)
}
