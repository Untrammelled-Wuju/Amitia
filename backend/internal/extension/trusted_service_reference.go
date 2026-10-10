package extension

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

func trustedServiceReference(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		query, err := url.ParseQuery(c.Request.URL.RawQuery)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service query"})
			return
		}
		serviceID := c.Param("serviceId")
		values, present := query["service_id"]
		if present {
			if len(values) != 1 || strings.TrimSpace(values[0]) == "" || serviceID != "" && serviceID != values[0] {
				c.JSON(http.StatusBadRequest, gin.H{"error": "service_id must be unique, nonempty and match the path"})
				return
			}
			if serviceID == "" {
				serviceID = values[0]
				c.Params = append(c.Params, gin.Param{Key: "serviceId", Value: serviceID})
			}
		}
		if strings.TrimSpace(serviceID) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "service_id required"})
			return
		}
		handler(c)
	}
}

func (api *TrustedServiceAPI) serviceOperationAlias(handler gin.HandlerFunc, legacyID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !c.Request.URL.Query().Has("service_id") {
			c.Params = append(c.Params, gin.Param{Key: "serviceId", Value: legacyID})
			trustedServiceReference(api.getService)(c)
			return
		}
		trustedServiceReference(handler)(c)
	}
}

func (api *TrustedServiceAPI) serviceListOrGet(c *gin.Context) {
	query, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service query"})
		return
	}
	if query.Has("service_id") {
		trustedServiceReference(api.getService)(c)
		return
	}
	api.listServices(c)
}
