package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/olivere/elastic"
)

type WebsiteCheck struct {
	TestId      string         `json:"testId"`
	WebsiteName string         `json:"websiteName"`
	Website     *WebsiteConfig `json:"-"`
	Passed      bool           `json:"passed"`
	Timestamp   time.Time      `json:"timestamp"`
}

func (result *WebsiteCheck) GetId() string {
	return fmt.Sprintf(
		"website:%v:%v",
		strings.ToLower(strings.Replace(result.GetWebsiteName(), " ", "-", -1)),
		result.Timestamp,
	)
}

func (result *WebsiteCheck) GetTestId() string {
	return fmt.Sprintf(
		"website:%v",
		strings.ToLower(strings.Replace(result.GetWebsiteName(), " ", "-", -1)),
	)
}

func (result *WebsiteCheck) GetWebsiteName() string {
	if result.Website == nil {
		return "-"
	}

	return result.Website.Name
}

func (result *WebsiteCheck) GetMapping(setTimestamp bool) (*string, error) {
	result.WebsiteName = result.GetWebsiteName()
	result.TestId = result.GetTestId()
	if setTimestamp {
		result.Timestamp = time.Now()
	}

	bytes, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	mapping := string(bytes)

	return &mapping, nil
}

func (result *WebsiteCheck) Save() error {
	bulkRequest := database.Bulk()
	mapping, err := result.GetMapping(true)
	if err != nil {
		return err
	}
	req := elastic.NewBulkIndexRequest().
		Index("website_check").
		Type("website_check").
		Id(result.GetId()).
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
		// if count, ok := errored["website_check"]; ok {
		// 	errorCount = count
		// }
		if count, ok := indexed["website_check"]; ok {
			indexCount = count
		}
		Debug("Indexed ", indexCount, " ", "website_check")
		// if errorCount > 0 {
		// 	Error(errorCount, " ", "website_check", " errors")
		// }
		database.Flush().Index("website_check").Do(ctx)

		if len(indexErrors) > 0 {
			return errors.New(fmt.Sprintf("There were problems indexing the result: %v", strings.Join(indexErrors, ", ")))
		}
	}

	return nil
}

func (checkResult *WebsiteCheck) GetSeverity() *SeverityConfig {
	var severityConfig *SeverityConfig
	if checkResult.Website != nil {
		severityConfig = checkResult.Website.Severity()
	}

	return severityConfig
}

func (checkResult *WebsiteCheck) GetSeverityName() string {
	var severityName string
	if checkResult.Website != nil {
		severityName = checkResult.Website.SeverityType
	}

	return severityName
}

func (checkResult *WebsiteCheck) CanResendAlert() bool {
	severityConfig := checkResult.GetSeverity()

	timeFrom := time.Now().Add(-severityConfig.AlertResendMinutes * time.Minute)
	results, err := checkResult.GetAlertsSince(timeFrom)
	if err != nil {
		Error("Failed to get alerts matching `", checkResult.GetTestId(), "`: ", err)

		return true
	}

	if len(*results) == 0 {
		return true
	}

	return false
}

func (checkResult *WebsiteCheck) IsSevere() bool {
	severityConfig := checkResult.GetSeverity()

	if severityConfig == nil {
		Error(fmt.Sprintf("No severity set for website `%v` - not sending alert", checkResult.GetWebsiteName()))

		return false
	}

	timeFrom := time.Now().Add(-severityConfig.CheckMinutes * time.Minute)
	results, err := checkResult.GetResultsSince(timeFrom)
	if err != nil {
		Error("Failed to get results matching `", checkResult.GetTestId(), "`: ", err)

		return true
	}

	hasOlder := false
	var failureCount float32 = 0
	var totalCount float32 = 0
	for _, result := range *results {
		if result.Timestamp.Before(timeFrom) {
			hasOlder = true
			continue
		}
		totalCount++
		if !result.Passed {
			failureCount++
		}
	}

	if hasOlder && (failureCount/totalCount)*100 > float32(severityConfig.FailedAttemptsPercentage) {
		return true
	}

	return false
}

func (result *WebsiteCheck) GetResultsSince(timeFrom time.Time) (*[]WebsiteCheck, error) {
	query := elastic.NewBoolQuery()
	query.Must(elastic.NewMatchQuery("testId", result.GetTestId()).Operator("AND")).
		Must(elastic.NewRangeQuery("timestamp").From(timeFrom.Add(-1 * time.Minute)).To(time.Now()))
	search, err := database.Search().
		Index("website_check").
		Query(query).
		Sort("timestamp", true).
		From(0).Size(1000).
		Do(ctx)

	if err != nil {
		return nil, errors.New(fmt.Sprintf("Could not get website results: %v", err))
	}

	results := make([]WebsiteCheck, 0)
	for _, record := range search.Hits.Hits {
		var result WebsiteCheck
		err = json.Unmarshal(*record.Source, &result)
		if err != nil {
			Error("Could not deserialise website config json: ", err)
			continue
		}

		results = append(results, result)
	}

	return &results, nil
}

func (result *WebsiteCheck) GetAlertsSince(timeFrom time.Time) (*[]Alert, error) {
	query := elastic.NewBoolQuery()
	query.Must(elastic.NewMatchQuery("alertId", result.GetTestId()).Operator("AND")).
		Must(elastic.NewRangeQuery("timestamp").From(timeFrom).To(time.Now()))
	search, err := database.Search().
		Index("alert").
		Query(query).
		Sort("timestamp", true).
		From(0).Size(1000).
		Do(ctx)

	if err != nil {
		return nil, errors.New(fmt.Sprintf("Could not get result alerts: %v", err))
	}

	results := make([]Alert, 0)
	for _, record := range search.Hits.Hits {
		var result Alert
		err = json.Unmarshal(*record.Source, &result)
		if err != nil {
			Error("Could not deserialise alert json: ", err)
			continue
		}

		results = append(results, result)
	}

	return &results, nil
}
