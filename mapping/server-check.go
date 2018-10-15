package mapping

const ServerCheck = `
{
	"settings": {
		"number_of_shards": 1,
		"number_of_replicas": 0
	},
	"mappings": {
		"server_check": {
			"properties": {
				"testId": {
					"type": "text"
				},
				"serverName": {
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
