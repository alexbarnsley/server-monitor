package mapping

const Alert = `
{
	"settings": {
		"number_of_shards": 1,
		"number_of_replicas": 0
	},
	"mappings": {
		"alert": {
			"properties": {
				"alertId": {
					"type": "text"
				},
				"timestamp": {
					"type": "date"
				}
			}
		}
	}
}`
