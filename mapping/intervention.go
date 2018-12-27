package mapping

const Intervention = `
{
	"settings": {
		"number_of_shards": 1,
		"number_of_replicas": 0
	},
	"mappings": {
		"intervention": {
			"properties": {
				"testId": {
					"type": "text"
				},
				"timestamp": {
					"type": "date"
				}
			}
		}
	}
}`