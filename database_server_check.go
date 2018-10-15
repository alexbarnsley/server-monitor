package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/olivere/elastic"
)

type ServerCheck struct {
	TestId     string        `json:"testId"`
	ServerName string        `json:"serverName"`
	CheckName  string        `json:"checkName"`
	Server     *ServerConfig `json:"-"`
	Check      *Check        `json:"-"`
	Passed     bool          `json:"passed"`
	Timestamp  time.Time     `json:"timestamp"`
}

func (result *ServerCheck) GetId() string {
	return fmt.Sprintf(
		"%v:%v:%v",
		strings.Replace(result.GetServerName(), " ", "-", -1),
		strings.Replace(result.GetCheckName(), " ", "-", -1),
		result.Timestamp,
	)
}

func (result *ServerCheck) GetTestId() string {
	return fmt.Sprintf(
		"%v:%v",
		strings.Replace(result.GetServerName(), " ", "-", -1),
		strings.Replace(result.GetCheckName(), " ", "-", -1),
	)
}

func (result *ServerCheck) GetServerName() string {
	if result.Server == nil {
		return "-"
	}

	return result.Server.Name
}

func (result *ServerCheck) GetCheckName() string {
	if result.Check == nil {
		return "-"
	}

	return result.Check.Name
}

func (result *ServerCheck) GetMapping(setTimestamp bool) (*string, error) {
	result.ServerName = result.GetServerName()
	result.CheckName = result.GetCheckName()
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

func (result *ServerCheck) Save() error {
	bulkRequest := database.Bulk()
	mapping, err := result.GetMapping(true)
	if err != nil {
		return err
	}
	req := elastic.NewBulkIndexRequest().
		Index("server_check").
		Type("server_check").
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
		// if count, ok := errored["server_check"]; ok {
		// 	errorCount = count
		// }
		if count, ok := indexed["server_check"]; ok {
			indexCount = count
		}
		Debug("Indexed ", indexCount, " ", "server_check")
		// if errorCount > 0 {
		// 	Error(errorCount, " ", "server_check", " errors")
		// }
		database.Flush().Index("server_check").Do(ctx)

		if len(indexErrors) > 0 {
			return errors.New(fmt.Sprintf("There were problems indexing the result: %v", strings.Join(indexErrors, ", ")))
		}
	}

	return nil
}

func (checkResult *ServerCheck) GetSeverity() *SeverityConfig {
	var severityConfig *SeverityConfig
	if checkResult.Server != nil {
		severityConfig = checkResult.Server.Severity()
	}
	if checkResult.Check != nil {
		checkSeverityConfig := checkResult.Check.Severity()
		if checkSeverityConfig != nil {
			severityConfig = checkSeverityConfig
		}
	}

	return severityConfig
}

func (checkResult *ServerCheck) GetSeverityName() string {
	var severityName string
	if checkResult.Server != nil {
		severityName = checkResult.Server.SeverityType
	}
	if checkResult.Check != nil {
		if checkResult.Check.Severity() != nil {
			severityName = checkResult.Check.SeverityType
		}
	}

	return severityName
}

func (checkResult *ServerCheck) IsSevere() bool {
	severityConfig := checkResult.GetSeverity()

	if severityConfig == nil {
		Error(fmt.Sprintf("No severity set for server `%v` or check `%v` - not sending alert", checkResult.GetServerName(), checkResult.GetCheckName()))

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

func (checkResult *ServerCheck) CanResendAlert() bool {
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

func (result *ServerCheck) GetResultsSince(timeFrom time.Time) (*[]ServerCheck, error) {
	query := elastic.NewBoolQuery()
	query.Must(elastic.NewMatchQuery("testId", result.GetTestId())).
		Must(elastic.NewRangeQuery("timestamp").From(timeFrom.Add(-1 * time.Minute)).To(time.Now()))
	search, err := database.Search().
		Index("server_check").
		Query(query).
		Sort("timestamp", true).
		From(0).Size(1000).
		Do(ctx)

	if err != nil {
		return nil, errors.New(fmt.Sprintf("Could not get server results: %v", err))
	}

	results := make([]ServerCheck, 0)
	for _, record := range search.Hits.Hits {
		var result ServerCheck
		err = json.Unmarshal(*record.Source, &result)
		if err != nil {
			Error("Could not deserialise server config json: ", err)
			continue
		}

		results = append(results, result)
	}

	return &results, nil
}

func (result *ServerCheck) GetAlertsSince(timeFrom time.Time) (*[]Alert, error) {
	query := elastic.NewBoolQuery()
	query.Must(elastic.NewMatchQuery("alertId", result.GetTestId())).
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
