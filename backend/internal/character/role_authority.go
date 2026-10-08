package character

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/character/card"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/pkg/util"
)

const RoleAuthorityHeader = "X-Amitia-Role-Authority"

type roleAuthoritySecret struct {
	once sync.Once
	key  [32]byte
	err  error
}

var roleAuthoritySecrets sync.Map

func (s *service) RoleAuthority(spaceID string) (string, error) {
	if s.db == nil {
		return "", errors.New("角色数据源不可用")
	}
	connection, err := s.db.DB()
	if err != nil {
		return "", err
	}
	value, _ := roleAuthoritySecrets.LoadOrStore(connection, &roleAuthoritySecret{})
	secret := value.(*roleAuthoritySecret)
	secret.once.Do(func() { _, secret.err = rand.Read(secret.key[:]) })
	if secret.err != nil {
		return "", secret.err
	}
	mac := hmac.New(sha256.New, secret.key[:])
	mac.Write([]byte("amitia-role-authority/v1\x00"))
	mac.Write([]byte(requestidentity.NormalizeSpaceID(spaceID)))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func (h *Handler) roleAuthority(c *gin.Context) (string, error) {
	source, ok := h.service.(interface{ RoleAuthority(string) (string, error) })
	if !ok {
		return "", errors.New("角色数据源不可用")
	}
	return source.RoleAuthority(requestidentity.ResolveGin(c))
}

func (h *Handler) Authority(c *gin.Context) {
	token, err := h.roleAuthority(c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "角色数据源不可用"})
		return
	}
	c.Header("Cache-Control", "no-store")
	util.SuccessResponse(c, gin.H{"roleAuthority": token})
}

func (h *Handler) guardRoleAuthority() gin.HandlerFunc {
	return func(c *gin.Context) {
		supplied := c.GetHeader(RoleAuthorityHeader)
		if strings.HasPrefix(strings.ToLower(c.ContentType()), "multipart/form-data") {
			limit := int64(card.MaxInputBytes + 1024*1024)
			if strings.HasSuffix(c.FullPath(), "/avatar") {
				limit = 8 * 1024 * 1024
			}
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
			if err := c.Request.ParseMultipartForm(1024 * 1024); err != nil {
				if c.Request.MultipartForm != nil {
					_ = c.Request.MultipartForm.RemoveAll()
				}
				c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"code": 413, "msg": "角色文件过大或上传格式无效"})
				return
			}
			if c.Request.MultipartForm != nil {
				defer c.Request.MultipartForm.RemoveAll()
			}
			field := c.Request.FormValue("roleAuthority")
			if supplied == "" {
				supplied = field
			} else if field != "" && field != supplied {
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": 409, "msg": "角色数据归属标识不一致，请重新加载角色"})
				return
			}
		}
		if supplied == "" {
			actor := security.GetActor(c)
			if actor != nil && actor.PrincipalType == auth.PrincipalTrustedDevice && c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": 409, "msg": "缺少角色数据归属标识，请重新加载角色后再操作"})
				return
			}
			c.Next()
			return
		}
		expected, err := h.roleAuthority(c)
		if err != nil || len(supplied) != 64 || supplied != strings.TrimSpace(supplied) || !hmac.Equal([]byte(supplied), []byte(expected)) {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": 409, "msg": "角色数据归属已变化，请重新加载角色后再操作"})
			return
		}
		c.Next()
	}
}

func (h *Handler) stampRoleAuthority(c *gin.Context, data any) {
	token, err := h.roleAuthority(c)
	if err != nil {
		return
	}
	switch item := data.(type) {
	case *CardPreviewResult:
		if item != nil {
			item.RoleAuthority = token
		}
	case *CardImportResult:
		if item != nil {
			item.RoleAuthority = token
		}
	case *Character:
		if item != nil {
			item.RoleAuthority = token
		}
	case []Character:
		for i := range item {
			item[i].RoleAuthority = token
		}
	case *CharacterTemplate:
		if item != nil {
			item.RoleAuthority = token
		}
	case []CharacterTemplate:
		for i := range item {
			item[i].RoleAuthority = token
		}
	case *RoleProfileResponse:
		if item != nil {
			item.RoleAuthority = token
		}
	}
}
