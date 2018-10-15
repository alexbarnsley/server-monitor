package main

import (
	"context"
	"fmt"

	"github.com/olivere/elastic"
	"server-monitor/mapping"
)

type index struct {
	name    string
	mapping string
}

var database *elastic.Client
var ctx = context.Background()
var indexes []index = []index{
	{
		name:    "server_check",
		mapping: mapping.ServerCheck,
	},
	{
		name:    "alert",
		mapping: mapping.Alert,
	},
}

func InitiateDatabase() {
	__connect()
	__createIndexes()
}

func __connect() {
	host := "127.0.0.1"
	port := 9200
	username := "elastic"
	password := "changeme"

	if config.Elastic.Host != "" {
		host = config.Elastic.Host
	}
	if config.Elastic.Port != 0 {
		port = config.Elastic.Port
	}
	if config.Elastic.Username != "" {
		username = config.Elastic.Username
	}
	if config.Elastic.Password != "" {
		password = config.Elastic.Password
	}

	var err error
	elastic.SetSniff(false)
	Info(fmt.Sprintf("http://%s:%d", host, port))
	database, err = elastic.NewClient(
		elastic.SetURL(fmt.Sprintf("http://%s:%d", host, port)),
		elastic.SetBasicAuth(username, password),
		elastic.SetSniff(false),
	)
	if err != nil || !database.IsRunning() {
		Fatal("Elastic - could not connect - ", err)
	}
}

func __createIndexes() {
	for _, index := range indexes {
		// if index.name == "alert" {
		// database.DeleteIndex(index.name).Do(ctx)
		// }
		exists, err := database.IndexExists(index.name).Do(ctx)
		if err != nil {
			Fatal("Elastic - could check for index `"+index.name+"`: ", err)
		}
		if !exists {
			createResponse, err := database.CreateIndex(index.name).BodyString(index.mapping).Do(ctx)
			if err != nil {
				Fatal("Elastic - could not create index "+index.name+": ", err)
			} else {
				Info("Elastic - index " + index.name + " created")
			}
			if !createResponse.Acknowledged {
				Info("Index "+index.name+" not Acknowledged 🤷‍♂️", err)
			}
		}
	}
}
