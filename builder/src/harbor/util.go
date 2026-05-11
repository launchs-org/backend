package harbor

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func decodeJSON(resp *http.Response, v any) error {
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("JSON デコード失敗: %w", err)
	}
	return nil
}
