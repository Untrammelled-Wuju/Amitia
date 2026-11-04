package extension

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

const trustedServiceIDContextKey = "trustedServiceID"

func trustedServiceReference(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		query, err := url.ParseQuery(c.Request.URL.RawQuery)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		values, exists := query["service_id"]
		if !exists || len(values) != 1 || strings.TrimSpace(values[0]) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "service_id required"})
			return
		}
		c.Set(trustedServiceIDContextKey, values[0])
		handler(c)
	}
}

func trustedServiceID(c *gin.Context) string {
	value, _ := c.Get(trustedServiceIDContextKey)
	serviceID, _ := value.(string)
	return serviceID
}

func (api *TrustedServiceAPI) serviceListOrGet(c *gin.Context) {
	query, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if query.Has("service_id") {
		trustedServiceReference(api.getService)(c)
		return
	}
	api.listServices(c)
}
