package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/olivere/elastic"
)

type Alert struct {
	AlertId   string    `json:"alertId"`
	Timestamp time.Time `json:"timestamp"`
}

func (alert *Alert) GetId() string {
	return fmt.Sprintf(
		"%v:%v",
		alert.AlertId,
		alert.Timestamp,
	)
}

func (alert *Alert) GetMapping(setTimestamp bool) (*string, error) {
	if setTimestamp {
		alert.Timestamp = time.Now()
	}

	bytes, err := json.Marshal(alert)

	if err != nil {
		return nil, err
	}

	mapping := string(bytes)

	return &mapping, nil
}

func (alert *Alert) Save() error {
	bulkRequest := database.Bulk()
	mapping, err := alert.GetMapping(true)
	if err != nil {
		return err
	}
	req := elastic.NewBulkIndexRequest().
		Index("alert").
		Type("alert").
		Id(alert.GetId()).
		Doc(mapping)
	bulkRequest = bulkRequest.Add(req)
	response, err := bulkRequest.Do(ctx)
	if err != nil {
		return errors.New(fmt.Sprintf("Could not process mappings to elastic: %v", err))
		// Error(bulkRequest)
	} else {
		indexed := make(map[string]int)
		indexErrors := make([]string, 0)
		for _, itemRecord := range response.Items {
			var item *elastic.BulkResponseItem = nil
			if indexResponse, ok := itemRecord["index"]; ok {
				item = indexResponse
			}
			if item == nil {
				continue
			}
			if item.Error == nil {
				indexed[item.Index]++
			} else {
				// errored[item.Index]++
				indexErrors = append(indexErrors, "`"+item.Index+"` ", item.Id, ": ", item.Error.Reason)
				// Error("Error for `"+item.Index+"` ", item.Id, ": ", item.Error.Reason)
			}
		}
		// errorCount := len
		indexCount := 0
		// if count, ok := errored["alert"]; ok {
		// 	errorCount = count
		// }
		if count, ok := indexed["alert"]; ok {
			indexCount = count
		}
		Debug("Indexed ", indexCount, " ", "alert")
		// if errorCount > 0 {
		// 	Error(errorCount, " ", "alert", " errors")
		// }
		database.Flush().Index("alert").Do(ctx)

		if len(indexErrors) > 0 {
			return errors.New(fmt.Sprintf("There were problems indexing the alert: %v", strings.Join(indexErrors, ", ")))
		}
	}

	return nil
}
