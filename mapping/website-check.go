package mapping

const WebsiteCheck = `
{
	"settings": {
		"number_of_shards": 1,
		"number_of_replicas": 0
	},
	"mappings": {
		"website_check": {
			"properties": {
				"testId": {
					"type": "text"
				},
				"checkName": {
					"type": "text"
				},
				"passed": {
					"type": "boolean"
				},
				"timestamp": {
					"type": "date"
				}
			}
		}
	}
}`
