package requestidentity

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const DefaultUserID = "default"

func ResolveGin(c *gin.Context, envelopeUserID string) string {
	if c != nil {
		if value, exists := c.Get("userID"); exists {
			if resolved := valueString(value); resolved != "" {
				return resolved
			}
		}
		if value, exists := c.Get("user_id"); exists {
			if resolved := valueString(value); resolved != "" {
				return resolved
			}
		}
	}
	if resolved := strings.TrimSpace(envelopeUserID); resolved != "" {
		return resolved
	}
	if c != nil {
		if resolved := strings.TrimSpace(c.GetHeader("X-User-ID")); resolved != "" {
			return resolved
		}
	}
	return DefaultUserID
}

func valueString(value interface{}) string {
	switch current := value.(type) {
	case string:
		return strings.TrimSpace(current)
	case int:
		return strconv.Itoa(current)
	case int64:
		return strconv.FormatInt(current, 10)
	case float64:
		return strconv.FormatInt(int64(current), 10)
	default:
		return ""
	}
}
