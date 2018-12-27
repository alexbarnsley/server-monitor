package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/olivere/elastic"
)

type Intervention struct {
	TestId    string    `json:"testId"`
	Timestamp time.Time `json:"timestamp"`
}

func (intervention *Intervention) GetId() string {
	return fmt.Sprintf(
		"%v:%v",
		intervention.TestId,
		intervention.Timestamp,
	)
}

func (intervention *Intervention) GetMapping(setTimestamp bool) (*string, error) {
	if setTimestamp {
		intervention.Timestamp = time.Now()
	}

	bytes, err := json.Marshal(intervention)
	if err != nil {
		return nil, err
	}

	mapping := string(bytes)

	return &mapping, nil
}

func (intervention *Intervention) Save() error {
	bulkRequest := database.Bulk()
	mapping, err := intervention.GetMapping(true)
	if err != nil {
		return err
	}
	req := elastic.NewBulkIndexRequest().
		Index("intervention").
		Type("intervention").
		Id(intervention.GetId()).
		Doc(mapping)
	bulkRequest = bulkRequest.Add(req)
	response, err := bulkRequest.Do(ctx)
	if err != nil {
		return errors.New(fmt.Sprintf("Could not process intervention mappings to elastic: %v", err))
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
				indexErrors = append(indexErrors, "`"+item.Index+"` ", item.Id, ": ", item.Error.Reason)
			}
		}
		indexCount := 0
		if count, ok := indexed["intervention"]; ok {
			indexCount = count
		}
		Debug("Indexed ", indexCount, " ", "intervention")
		database.Flush().Index("intervention").Do(ctx)

		if len(indexErrors) > 0 {
			return errors.New(fmt.Sprintf("There were problems indexing the intervention: %v", strings.Join(indexErrors, ", ")))
		}
	}

	return nil
}
