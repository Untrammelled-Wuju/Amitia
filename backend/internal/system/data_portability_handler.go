package system

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/system/dataportability"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) DataPortabilityPreviewExport(c *gin.Context) {
	var body struct {
		Scope       string `json:"scope"`
		Profile     string `json:"profile"`
		CharacterID string `json:"characterId"`
		Passphrase  string `json:"passphrase"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	svc, ok := h.service.(*service)
	if !ok {
		util.ErrorResponse(c, http.StatusServiceUnavailable, "数据可迁移服务不可用", nil)
		return
	}
	result, err := svc.PreviewDataPortabilityExport(c.Request.Context(), ExportRequest{
		Scope:       dataportability.BackupScope(strings.TrimSpace(body.Scope)),
		Profile:     dataportability.BackupProfile(strings.TrimSpace(body.Profile)),
		CharacterID: strings.TrimSpace(body.CharacterID),
		Passphrase:  body.Passphrase,
	})
	if err != nil {
		respondDataPortabilityError(c, err)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) DataPortabilityExport(c *gin.Context) {
	var body struct {
		Scope       string `json:"scope"`
		Profile     string `json:"profile"`
		CharacterID string `json:"characterId"`
		Passphrase  string `json:"passphrase"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	svc, ok := h.service.(*service)
	if !ok {
		util.ErrorResponse(c, http.StatusServiceUnavailable, "数据可迁移服务不可用", nil)
		return
	}
	result, err := svc.ExportDataPortability(c.Request.Context(), ExportRequest{
		Scope:       dataportability.BackupScope(strings.TrimSpace(body.Scope)),
		Profile:     dataportability.BackupProfile(strings.TrimSpace(body.Profile)),
		CharacterID: strings.TrimSpace(body.CharacterID),
		Passphrase:  body.Passphrase,
	})
	if err != nil {
		respondDataPortabilityError(c, err)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) DataPortabilityExportDownload(c *gin.Context) {
	svc, ok := h.service.(*service)
	if !ok {
		util.ErrorResponse(c, http.StatusServiceUnavailable, "数据可迁移服务不可用", nil)
		return
	}
	path, err := svc.DataPortabilityExportPath(c.Param("filename"))
	if err != nil {
		respondDataPortabilityError(c, err)
		return
	}
	c.FileAttachment(path, filepath.Base(path))
}

func (h *Handler) DataPortabilityInspectImport(c *gin.Context) {
	if h.service == nil {
		util.ErrorResponse(c, http.StatusServiceUnavailable, "数据可迁移服务不可用", nil)
		return
	}
	svc, ok := h.service.(*service)
	if !ok {
		util.ErrorResponse(c, http.StatusServiceUnavailable, "数据可迁移服务不可用", nil)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, dataportability.MaxPackageSizeBytes)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, "缺少导入归档", nil)
		return
	}
	defer file.Close()
	uploadDir := filepath.Join(svc.dataDir, "imports", ".uploads")
	if err := os.MkdirAll(uploadDir, 0o700); err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	extension := ".amitia-backup"
	if strings.HasSuffix(strings.ToLower(header.Filename), ".enc") {
		extension = ".enc"
	}
	uploadPath := filepath.Join(uploadDir, uuid.NewString()+extension)
	output, err := os.OpenFile(uploadPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if _, err := io.Copy(output, file); err != nil {
		output.Close()
		_ = os.Remove(uploadPath)
		util.ErrorResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if err := output.Close(); err != nil {
		_ = os.Remove(uploadPath)
		util.ErrorResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	result, err := svc.InspectDataPortabilityImport(c.Request.Context(), uploadPath, c.PostForm("passphrase"))
	if err != nil {
		_ = os.Remove(uploadPath)
		respondDataPortabilityError(c, err)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) DataPortabilityCommitImport(c *gin.Context) {
	svc, ok := h.service.(*service)
	if !ok {
		util.ErrorResponse(c, http.StatusServiceUnavailable, "数据可迁移服务不可用", nil)
		return
	}
	var body struct {
		Policy   string `json:"policy"`
		Activate bool   `json:"activate"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	result, err := svc.CommitDataPortabilityImport(
		c.Request.Context(),
		c.Param("ticketId"),
		dataportability.CollisionPolicy(strings.TrimSpace(body.Policy)),
		body.Activate,
	)
	if err != nil {
		respondDataPortabilityError(c, err)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) DataPortabilityCancelImport(c *gin.Context) {
	svc, ok := h.service.(*service)
	if !ok {
		util.ErrorResponse(c, http.StatusServiceUnavailable, "数据可迁移服务不可用", nil)
		return
	}
	if err := svc.CancelDataPortabilityImport(c.Param("ticketId")); err != nil {
		respondDataPortabilityError(c, err)
		return
	}
	util.SuccessResponse(c, map[string]interface{}{"cancelled": true})
}

func (h *Handler) DataPortabilityBackups(c *gin.Context) {
	svc, ok := h.service.(*service)
	if !ok {
		util.ErrorResponse(c, http.StatusServiceUnavailable, "数据可迁移服务不可用", nil)
		return
	}
	result, err := svc.ListDataPortabilityBackups()
	if err != nil {
		respondDataPortabilityError(c, err)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) DataPortabilityCreateBackup(c *gin.Context) {
	svc, ok := h.service.(*service)
	if !ok {
		util.ErrorResponse(c, http.StatusServiceUnavailable, "数据可迁移服务不可用", nil)
		return
	}
	var body struct {
		Passphrase string `json:"passphrase"`
	}
	if err := c.ShouldBindJSON(&body); err != nil && !errors.Is(err, io.EOF) {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	result, err := svc.CreateDataPortabilityBackup(c.Request.Context(), body.Passphrase)
	if err != nil {
		respondDataPortabilityError(c, err)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) DataPortabilityPreviewRestore(c *gin.Context) {
	svc, ok := h.service.(*service)
	if !ok {
		util.ErrorResponse(c, http.StatusServiceUnavailable, "数据可迁移服务不可用", nil)
		return
	}
	var body struct {
		Passphrase string `json:"passphrase"`
	}
	if err := c.ShouldBindJSON(&body); err != nil && !errors.Is(err, io.EOF) {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	result, err := svc.PreviewDataPortabilityRestore(c.Request.Context(), c.Param("name"), body.Passphrase)
	if err != nil {
		respondDataPortabilityError(c, err)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) DataPortabilityRestore(c *gin.Context) {
	svc, ok := h.service.(*service)
	if !ok {
		util.ErrorResponse(c, http.StatusServiceUnavailable, "数据可迁移服务不可用", nil)
		return
	}
	result, err := svc.ExecuteDataPortabilityRestore(c.Request.Context(), c.Param("operationId"))
	if err != nil {
		respondDataPortabilityErrorWithResult(c, err, result)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) DataPortabilityCancelRestore(c *gin.Context) {
	svc, ok := h.service.(*service)
	if !ok {
		util.ErrorResponse(c, http.StatusServiceUnavailable, "数据可迁移服务不可用", nil)
		return
	}
	if err := svc.CancelDataPortabilityRestore(); err != nil {
		respondDataPortabilityError(c, err)
		return
	}
	util.SuccessResponse(c, map[string]interface{}{"cancelled": true})
}

func (h *Handler) DataPortabilityDeleteBackup(c *gin.Context) {
	svc, ok := h.service.(*service)
	if !ok {
		util.ErrorResponse(c, http.StatusServiceUnavailable, "数据可迁移服务不可用", nil)
		return
	}
	if err := svc.DeleteDataPortabilityBackup(c.Param("name")); err != nil {
		respondDataPortabilityError(c, err)
		return
	}
	util.SuccessResponse(c, map[string]interface{}{"deleted": true})
}

func respondDataPortabilityError(c *gin.Context, err error) {
	respondDataPortabilityErrorWithResult(c, err, nil)
}

func respondDataPortabilityErrorWithResult(c *gin.Context, err error, result map[string]interface{}) {
	status := http.StatusUnprocessableEntity
	switch {
	case errors.Is(err, dataportability.ErrBackupNotFound), errors.Is(err, dataportability.ErrImportTicketNotFound):
		status = http.StatusNotFound
	case errors.Is(err, dataportability.ErrBackupPassphraseRequired), errors.Is(err, dataportability.ErrRestoreDecryptFailed):
		status = http.StatusUnauthorized
	case errors.Is(err, dataportability.ErrImportTicketExpired), errors.Is(err, dataportability.ErrImportTicketChanged):
		status = http.StatusConflict
	case errors.Is(err, dataportability.ErrBackupScopeInvalid),
		errors.Is(err, dataportability.ErrBackupProfileInvalid),
		errors.Is(err, dataportability.ErrBackupPassphraseWeak),
		errors.Is(err, dataportability.ErrImportCollisionPolicyInvalid),
		errors.Is(err, dataportability.ErrRestoreSchemaUnsupported),
		errors.Is(err, dataportability.ErrImportFormatUnsupported):
		status = http.StatusBadRequest
	}
	util.ErrorResponse(c, status, err.Error(), result)
}
