package git

import (
	"encoding/json"
)

func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
