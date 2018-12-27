package main

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

var router = mux.NewRouter()

var server = &http.Server{
	Handler:      router,
	WriteTimeout: 15 * time.Second,
	ReadTimeout:  15 * time.Second,
}

func StartApi() {
	server.SetKeepAlivesEnabled(false)
	server.Addr = fmt.Sprintf("%v:%v", "0.0.0.0", "8080")

	router.HandleFunc("/status", Get_StatusHandler)

	logger.Fatal(server.ListenAndServe())
}

type StatusHandlerResponse struct {
	Running bool
	Failing int32
}

func Get_StatusHandler(w http.ResponseWriter, r *http.Request) {
	testedServers := 0
	failingServers := 0
	for i := 0; i < len(servers); i++ {
		server := &servers[i]
		if !server.Enabled {
			continue
		}

		results, err := checkResult.GetResultsSince(timeFrom)
		if err != nil {
			failingServers++
			continue
		}

		testedServers++
	}

	statusCode := 200
	if testedServers > 0 {
		statusCode = 500
	}
	response := &StatusHandlerResponse{
		Running: testedServers > 0,
		Failing: failingServers,
	}

	__sendResponse(w, r, statusCode, response, nil)
}

func __sendResponse(w http.ResponseWriter, r *http.Request, statusCode int, response interface{}, err *error) {
	responseJson, jsonErr := json.Marshal(response)
	if jsonErr != nil || response == nil {
		statusCode = 500
		responseJson, _ = json.Marshal(map[string]bool{
			"success": false,
		})
		if jsonErr != nil {
			logger.Error("API ", r.URL.String(), " - ", jsonErr)
		}
	}
	if err != nil {
		logger.Error("API ", r.URL.String(), " - ", *err)
	}

	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJson)
}
