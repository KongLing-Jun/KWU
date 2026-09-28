package airclaude

import (
	"fmt"
	"strings"
)

func validateAPIKey(apiKey string) (string, error) {
	apiKey = strings.TrimSpace(apiKey)
	if !strings.HasPrefix(apiKey, "sk-") || len(apiKey) <= len("sk-") {
		return "", fmt.Errorf("airclaude credential is unavailable")
	}
	return apiKey, nil
}
