package extension

import (
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

func registerSelfHostedExtensionRegistryRoutes(group *gin.RouterGroup, runtime *Runtime) {
	group.GET("/extensions/registry/api/v1/extensions/:extensionKey/latest", func(c *gin.Context) {
		serveSelfHostedRegistryIndex(c, runtime)
	})
	group.GET("/extensions/registry/packages/:extensionKey/:artifact", func(c *gin.Context) {
		serveSelfHostedRegistryArtifact(c, runtime)
	})
}

func serveSelfHostedRegistryIndex(c *gin.Context, runtime *Runtime) {
	root, extensionID, ok := selfHostedRegistryTarget(c, runtime)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	key := base64.RawURLEncoding.EncodeToString([]byte(extensionID))
	path := filepath.Join(root, "indexes", key+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/json; charset=utf-8", data)
}

func serveSelfHostedRegistryArtifact(c *gin.Context, runtime *Runtime) {
	root, extensionID, ok := selfHostedRegistryTarget(c, runtime)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	artifact := filepath.Base(c.Param("artifact"))
	if artifact != c.Param("artifact") || strings.HasPrefix(artifact, ".") {
		c.Status(http.StatusNotFound)
		return
	}
	key := base64.RawURLEncoding.EncodeToString([]byte(extensionID))
	path := filepath.Join(root, "packages", key, artifact)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.File(path)
}

func selfHostedRegistryTarget(c *gin.Context, runtime *Runtime) (string, string, bool) {
	if runtime == nil || runtime.Kernel == nil {
		return "", "", false
	}
	container := runtime.Kernel.Container()
	if container == nil || strings.TrimSpace(container.ExtRoot) == "" {
		return "", "", false
	}
	key := c.Param("extensionKey")
	decoded, err := base64.RawURLEncoding.DecodeString(key)
	if err != nil || len(decoded) == 0 || !utf8.Valid(decoded) {
		return "", "", false
	}
	extensionID := string(decoded)
	if strings.TrimSpace(extensionID) != extensionID || strings.ContainsAny(extensionID, "\x00\r\n") {
		return "", "", false
	}
	return filepath.Join(container.ExtRoot, "extension-registry"), extensionID, true
}
