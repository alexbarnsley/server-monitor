package main

import (
	"fmt"
)

func AlertSimplePush(subject string, message string) {
	httpClient.R().Get(fmt.Sprintf("https://api.simplepush.io/send/%s/%s/%s", config.Alerts.SimplePush.Code, subject, message))
}
