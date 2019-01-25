package main

import (
	"encoding/json"
	"fmt"
	"net/url"
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
		"pushBullet": false,
	}
	if serverResult != nil && server != nil {
		isSevere = serverResult != nil && serverResult.IsSevere()
		if isSevere && !serverResult.CanResendAlert() {
			return
		}
		if server.CanSendAlert("simplePush", config.Alerts.SimplePush.Default) {
			canSend["simplePush"] = true
		}
		if server.CanSendAlert("pushBullet", config.Alerts.PushBullet.Default) {
			canSend["pushBullet"] = true
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
		if website.CanSendAlert("pushBullet", config.Alerts.PushBullet.Default) {
			canSend["pushBullet"] = true
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
	if config.Alerts.PushBullet.Enabled && canSend["pushBullet"] {
		AlertPushBullet(subject, message)
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
	_, err := httpClient.R().Get(
		fmt.Sprintf(
			"https://api.simplepush.io/send/%s/%s/%s",
			config.Alerts.SimplePush.Code,
			url.QueryEscape(subject),
			url.QueryEscape(message),
		),
	)
	if err != nil {
		Error("Could not send SimplePush alert: ", err)
	}
	// spew.Dump(response)
}

func AlertPushBullet(subject string, message string) {
	body := map[string]string{
		"type":  "note",
		"title": subject,
		"body":  message,
		"email": config.Alerts.PushBullet.Email,
	}
	jsonBody, jsonErr := json.Marshal(body)

	if jsonErr != nil {
		Error("Could not send PushBullet alert: ", jsonErr)

		return
	}

	_, err := httpClient.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Access-Token", config.Alerts.PushBullet.AccessToken).
		SetBody(jsonBody).
		Post("https://api.pushbullet.com/v2/pushes")

	if err != nil {
		Error("Could not send PushBullet alert: ", err)
	}
	// spew.Dump(response)
}
