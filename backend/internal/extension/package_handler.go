//go:build legacy_migration

package extension

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
)

type PackageHandler struct {
	service  *PackageService
	problems *Handler
}

func NewPackageHandler(service *PackageService, problems *Handler) *PackageHandler {
	return &PackageHandler{service: service, problems: problems}
}

func (h *PackageHandler) Preview(c *gin.Context) {
	preview, ok := h.previewStream(c)
	if !ok {
		return
	}
	success(c, preview)
}

func (h *PackageHandler) PreviewUpgrade(c *gin.Context) {
	preview, ok := h.previewStream(c)
	if !ok {
		return
	}
	if preview.ID != c.Param("id") || preview.Conflict != PackageConflictUpgrade {
		h.problems.problem(c, NewExtensionError(ErrPackageVersionConflict, "所选包不是该扩展的有效升级", preview.ID, false, nil))
		return
	}
	success(c, preview)
}

func (h *PackageHandler) previewStream(c *gin.Context) (PackageImportPreview, bool) {
	var empty PackageImportPreview
	if strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		h.problems.problem(c, NewExtensionError(ErrPackageFormatUnsupported, "通用扩展包接口只接受 Manifest v2 .amitiax；AgentSkills 请使用专用接口", "", false, nil))
		return empty, false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, DefaultPackageLimits().MaxExpandedBytes+(1<<20))
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		h.problems.problem(c, NewExtensionError(ErrPackageArchiveLimit, "上传内容超过限制或格式无效", "", false, err))
		return empty, false
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		h.problems.problem(c, NewExtensionError(ErrPackageInvalidArchive, "缺少扩展包文件", "", false, err))
		return empty, false
	}
	defer file.Close()
	preview, err := h.service.PreviewImportStream(c.Request.Context(), fmt.Sprint(c.GetInt(authenticatedUserKey)), c.Request.FormValue("scopeType"), c.Request.FormValue("scopeId"), header.Filename, file)
	if err != nil {
		h.problems.problem(c, err)
		return empty, false
	}
	return preview, true
}

func (h *PackageHandler) previewRequest(c *gin.Context) (PreviewPackageImportRequest, bool) {
	request := PreviewPackageImportRequest{UserID: fmt.Sprint(c.GetInt(authenticatedUserKey)), ScopeType: c.PostForm("scopeType"), ScopeID: c.PostForm("scopeId")}
	if strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		var body struct {
			ScopeType string `json:"scopeType"`
			ScopeID   string `json:"scopeId"`
			RootName  string `json:"rootName"`
			Files     []struct {
				Path   string `json:"path"`
				Base64 string `json:"base64"`
			} `json:"files"`
		}
		if c.ShouldBindJSON(&body) != nil {
			h.problems.problem(c, NewExtensionError(ErrPackageInvalidArchive, "目录导入请求无效", "", false, nil))
			return request, false
		}
		request.ScopeType, request.ScopeID, request.RootName = body.ScopeType, body.ScopeID, body.RootName
		request.Directory = map[string][]byte{}
		var total int64
		for _, file := range body.Files {
			content, err := base64.StdEncoding.DecodeString(file.Base64)
			if err != nil {
				h.problems.problem(c, NewExtensionError(ErrPackageInvalidArchive, "目录文件 Base64 无效", file.Path, false, err))
				return request, false
			}
			total += int64(len(content))
			if total > DefaultPackageLimits().MaxExpandedBytes {
				h.problems.problem(c, NewExtensionError(ErrPackageArchiveLimit, "目录导入超过大小限制", "", false, nil))
				return request, false
			}
			request.Directory[file.Path] = content
		}
		return request, true
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, DefaultPackageLimits().MaxExpandedBytes)
	if err := c.Request.ParseMultipartForm(DefaultPackageLimits().MaxExpandedBytes); err != nil {
		h.problems.problem(c, NewExtensionError(ErrPackageArchiveLimit, "上传内容超过限制或格式无效", "", false, err))
		return request, false
	}
	if request.ScopeType == "" {
		request.ScopeType = c.Request.FormValue("scopeType")
		request.ScopeID = c.Request.FormValue("scopeId")
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		h.problems.problem(c, NewExtensionError(ErrPackageInvalidArchive, "缺少扩展包文件", "", false, err))
		return request, false
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, DefaultPackageLimits().MaxExpandedBytes+1))
	if err != nil || int64(len(raw)) > DefaultPackageLimits().MaxExpandedBytes {
		h.problems.problem(c, NewExtensionError(ErrPackageArchiveLimit, "扩展包读取失败或超过限制", "", false, err))
		return request, false
	}
	request.FileName = header.Filename
	request.Raw = raw
	return request, true
}

func (h *PackageHandler) Install(c *gin.Context) {
	h.install(c, "")
}

func (h *PackageHandler) Session(c *gin.Context) {
	preview, err := h.service.GetImportSession(c.Request.Context(), c.Param("sessionId"), fmt.Sprint(c.GetInt(authenticatedUserKey)), c.Query("scopeType"), c.Query("scopeId"))
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, preview)
}

func (h *PackageHandler) CancelSession(c *gin.Context) {
	if err := h.service.CancelImportSession(c.Request.Context(), c.Param("sessionId"), fmt.Sprint(c.GetInt(authenticatedUserKey)), c.Query("scopeType"), c.Query("scopeId")); err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, gin.H{"cancelled": true})
}

func (h *PackageHandler) Upgrade(c *gin.Context) {
	h.install(c, c.Param("id"))
}

func (h *PackageHandler) install(c *gin.Context, expectedID string) {
	var request InstallPackageRequest
	if c.ShouldBindJSON(&request) != nil {
		h.problems.problem(c, NewExtensionError(ErrPackageInstallFailed, "安装请求无效", "", false, nil))
		return
	}
	request.UserID = fmt.Sprint(c.GetInt(authenticatedUserKey))
	request.ExpectedExtensionID = expectedID
	result, err := h.service.Install(c.Request.Context(), request)
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, result)
}

func (h *PackageHandler) Export(c *gin.Context) {
	var request ExportPackageRequest
	if c.ShouldBindJSON(&request) != nil {
		h.problems.problem(c, NewExtensionError(ErrPackageExportNotAllowed, "导出请求无效", "", false, nil))
		return
	}
	request.UserID = fmt.Sprint(c.GetInt(authenticatedUserKey))
	request.ExtensionID = c.Param("id")
	exported, err := h.service.Export(c.Request.Context(), request)
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, exported)
}

func (h *PackageHandler) Download(c *gin.Context) {
	exported, err := h.service.GetExport(c.Request.Context(), fmt.Sprint(c.GetInt(authenticatedUserKey)), c.Param("id"), c.Param("exportId"))
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	c.Header("Content-Type", exported.MIME)
	c.Header("Content-Disposition", `attachment; filename="`+safePackageFileName(exported.FileName)+`"`)
	c.Header("X-Content-Type-Options", "nosniff")
	c.FileAttachment(exported.LocalPath, safePackageFileName(exported.FileName))
}

func (h *PackageHandler) Versions(c *gin.Context) {
	items, err := h.service.ListVersions(c.Request.Context(), c.Param("id"), fmt.Sprint(c.GetInt(authenticatedUserKey)), c.Query("scopeType"), c.Query("scopeId"))
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, items)
}

func (h *PackageHandler) Compare(c *gin.Context) {
	diff, err := h.service.CompareVersions(c.Request.Context(), c.Param("id"), fmt.Sprint(c.GetInt(authenticatedUserKey)), c.Query("scopeType"), c.Query("scopeId"), c.Query("from"), c.Query("to"))
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, diff)
}

func (h *PackageHandler) Rollback(c *gin.Context) {
	var request struct {
		ScopeType         string `json:"scopeType"`
		ScopeID           string `json:"scopeId"`
		ConfirmationToken string `json:"confirmationToken"`
	}
	_ = c.ShouldBindJSON(&request)
	result, err := h.service.Rollback(c.Request.Context(), c.Param("id"), c.Param("version"), fmt.Sprint(c.GetInt(authenticatedUserKey)), request.ScopeType, request.ScopeID, request.ConfirmationToken)
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, result)
}

func (h *PackageHandler) Dependencies(c *gin.Context) {
	result, err := h.service.Dependencies(c.Request.Context(), c.Param("id"), fmt.Sprint(c.GetInt(authenticatedUserKey)), c.Query("scopeType"), c.Query("scopeId"))
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, result)
}

func (h *PackageHandler) Uninstall(c *gin.Context) {
	var req struct {
		ConfirmationToken string `json:"confirmationToken"`
	}
	_ = c.ShouldBindJSON(&req)
	result, err := h.service.Uninstall(c.Request.Context(), c.Param("id"), fmt.Sprint(c.GetInt(authenticatedUserKey)), c.Query("scopeType"), c.Query("scopeId"), req.ConfirmationToken)
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, result)
}

func (h *PackageHandler) PreviewUninstall(c *gin.Context) {
	result, err := h.service.PreviewUninstall(c.Request.Context(), c.Param("id"), fmt.Sprint(c.GetInt(authenticatedUserKey)), c.Query("scopeType"), c.Query("scopeId"))
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, result)
}

func (h *PackageHandler) PreviewRollback(c *gin.Context) {
	result, err := h.service.PreviewRollback(c.Request.Context(), c.Param("id"), c.Param("version"), fmt.Sprint(c.GetInt(authenticatedUserKey)), c.Query("scopeType"), c.Query("scopeId"))
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, result)
}

func (h *PackageHandler) ConfirmRollback(c *gin.Context) {
	var request struct {
		ExtensionID      string          `json:"extensionId"`
		TargetVersion    string          `json:"targetVersion"`
		ScopeType        string          `json:"scopeType"`
		ScopeID          string          `json:"scopeId"`
		PreviewSessionID string          `json:"previewSessionId"`
		Confirmations    map[string]bool `json:"confirmations"`
	}
	_ = c.ShouldBindJSON(&request)
	result, err := h.service.ConfirmRollback(c.Request.Context(), kernelruntime.PackageRollbackConfirmationRequest{
		ExtensionID:      request.ExtensionID,
		UserID:           fmt.Sprint(c.GetInt(authenticatedUserKey)),
		ScopeType:        request.ScopeType,
		ScopeID:          request.ScopeID,
		TargetVersion:    request.TargetVersion,
		PreviewSessionID: request.PreviewSessionID,
		Confirmations:    request.Confirmations,
	})
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, result)
}

func (h *PackageHandler) VerifyFinalGate(c *gin.Context) {
	result, err := h.service.VerifyOperationFinalGate(c.Request.Context(), c.Param("operationId"))
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, result)
}

func (h *PackageHandler) Operations(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	items, err := h.service.ListOperations(c.Request.Context(), fmt.Sprint(c.GetInt(authenticatedUserKey)), limit)
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, items)
}

func (h *PackageHandler) Operation(c *gin.Context) {
	item, err := h.service.GetOperation(c.Request.Context(), fmt.Sprint(c.GetInt(authenticatedUserKey)), c.Param("operationId"))
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, item)
}

func (h *PackageHandler) Signers(c *gin.Context) {
	items, err := h.service.ListSigners(c.Request.Context())
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, items)
}

func (h *PackageHandler) TrustSigner(c *gin.Context) {
	var request struct {
		PublisherID string `json:"publisherId"`
		KeyID       string `json:"keyId"`
		PublicKey   string `json:"publicKey"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&request); err != nil {
			h.problems.problem(c, NewExtensionError(ErrPackageSignatureInvalid, "发布者密钥请求无效", "", false, err))
			return
		}
	}
	if request.PublicKey != "" {
		publicKey, err := base64.StdEncoding.DecodeString(request.PublicKey)
		if err != nil {
			h.problems.problem(c, NewExtensionError(ErrPackageSignatureInvalid, "发布者公钥 Base64 无效", "", false, err))
			return
		}
		if err := h.service.RegisterSigner(c.Request.Context(), c.Param("fingerprint"), request.PublisherID, request.KeyID, publicKey); err != nil {
			h.problems.problem(c, err)
			return
		}
	}
	if err := h.service.TrustSigner(c.Request.Context(), c.Param("fingerprint")); err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, gin.H{"trusted": true})
}

func (h *PackageHandler) UntrustSigner(c *gin.Context) {
	if err := h.service.UntrustSigner(c.Request.Context(), c.Param("fingerprint")); err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, gin.H{"trusted": false})
}

func (h *PackageHandler) Metrics(c *gin.Context) {
	success(c, h.service.Metrics())
}
