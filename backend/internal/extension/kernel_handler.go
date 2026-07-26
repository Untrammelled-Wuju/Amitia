package extension

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func registerExtensionPackageRoutes(group *gin.RouterGroup, runtime *Runtime) {
	group.GET("/packages/status", func(c *gin.Context) {
		if runtime == nil || runtime.Kernel == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"ready": false})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ready": true, "root": runtime.Kernel.Root(), "installed": len(runtime.Kernel.List())})
	})
	group.GET("/packages", func(c *gin.Context) {
		if runtime == nil || runtime.Kernel == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "extension package service unavailable"})
			return
		}
		c.JSON(http.StatusOK, runtime.Kernel.List())
	})
	group.POST("/packages/install", func(c *gin.Context) {
		if runtime == nil || runtime.Kernel == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "extension package service unavailable"})
			return
		}
		file, err := c.FormFile("package")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "package file required"})
			return
		}
		temp, err := os.CreateTemp("", "amitiax-*.amitiax")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		path := temp.Name()
		_ = temp.Close()
		defer os.Remove(path)
		if err := c.SaveUploadedFile(file, path); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		installed, err := runtime.Kernel.Install(c.Request.Context(), path)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, installed)
	})
}
