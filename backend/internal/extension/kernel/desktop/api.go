package desktop

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type DesktopAPI struct {
	host *DesktopHost
}

func NewDesktopAPI(host *DesktopHost) *DesktopAPI {
	return &DesktopAPI{host: host}
}

func (api *DesktopAPI) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/extensions/:extensionId/desktop", api.ListByExtension)
	group.GET("/extensions/desktop/contributions/:contributionId", api.GetContributionByID)
	group.POST("/extensions/desktop/contributions/:contributionId/enable", api.EnableContributionByID)
	group.POST("/extensions/desktop/contributions/:contributionId/disable", api.DisableContributionByID)
	group.POST("/extensions/desktop/contributions/:contributionId/invoke", api.InvokeContributionAction)
	group.POST("/extensions/desktop/shortcuts/:contributionId/rebind", api.RebindShortcutByID)
	group.GET("/extensions/desktop/conflicts", api.ListAllConflicts)
	group.POST("/extensions/desktop/conflicts/:conflictId/resolve", api.ResolveConflictByID)
	group.GET("/extensions/desktop/snapshot", api.GetCurrentSnapshot)
	group.POST("/extensions/desktop/snapshot/build", api.BuildNewSnapshot)
	group.GET("/extensions/desktop/contracts", api.ListAllContracts)
	group.GET("/extensions/desktop/permissions", api.ListAllPermissions)
	group.GET("/extensions/desktop/resources", api.ListAllResources)
	group.GET("/extensions/desktop/circuit/status", api.CircuitStatus)
	group.POST("/extensions/desktop/circuit/reset", api.CircuitReset)
}

func apiError(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{"code": code, "message": message})
}

func (api *DesktopAPI) ListByExtension(c *gin.Context) {
	if api.host == nil {
		apiError(c, http.StatusServiceUnavailable, "desktop host unavailable")
		return
	}
	extID := c.Param("extensionId")
	contribs := api.host.ListByExtension(extID)
	c.JSON(http.StatusOK, gin.H{"items": contribs, "total": len(contribs)})
}

func (api *DesktopAPI) ListAllContributions(c *gin.Context) {
	if api.host == nil {
		apiError(c, http.StatusServiceUnavailable, "desktop host unavailable")
		return
	}
	contribs := api.host.ListContributions()
	c.JSON(http.StatusOK, gin.H{"items": contribs, "total": len(contribs)})
}

func (api *DesktopAPI) GetContributionByID(c *gin.Context) {
	if api.host == nil {
		apiError(c, http.StatusServiceUnavailable, "desktop host unavailable")
		return
	}
	contribID := c.Param("contributionId")
	contrib, ok := api.host.GetContribution(contribID)
	if !ok {
		apiError(c, http.StatusNotFound, fmt.Sprintf("contribution %s not found", contribID))
		return
	}
	c.JSON(http.StatusOK, contrib)
}

func (api *DesktopAPI) EnableContributionByID(c *gin.Context) {
	if api.host == nil {
		apiError(c, http.StatusServiceUnavailable, "desktop host unavailable")
		return
	}
	contribID := c.Param("contributionId")
	if err := api.host.EnableContribution(contribID); err != nil {
		apiError(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"contributionId": contribID, "status": ContributionStatusRegistered})
}

func (api *DesktopAPI) DisableContributionByID(c *gin.Context) {
	if api.host == nil {
		apiError(c, http.StatusServiceUnavailable, "desktop host unavailable")
		return
	}
	contribID := c.Param("contributionId")
	if err := api.host.DisableContribution(contribID); err != nil {
		apiError(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"contributionId": contribID, "status": ContributionStatusDisabled})
}

func (api *DesktopAPI) InvokeContributionAction(c *gin.Context) {
	if api.host == nil {
		apiError(c, http.StatusServiceUnavailable, "desktop host unavailable")
		return
	}
	contribID := c.Param("contributionId")
	var req struct {
		CharacterID    string          `json:"characterId,omitempty"`
		ConversationID string          `json:"conversationId,omitempty"`
		ExtensionID    string          `json:"extensionId,omitempty"`
		Global         bool            `json:"global,omitempty"`
		Input          json.RawMessage `json:"input,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	scopeCtx := ScopeContext{
		CharacterID:    req.CharacterID,
		ConversationID: req.ConversationID,
		ExtensionID:    req.ExtensionID,
		Global:         req.Global,
	}
	result, err := api.host.InvokeAction(c.Request.Context(), contribID, scopeCtx)
	if err != nil {
		apiError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"contributionId": contribID, "result": json.RawMessage(result)})
}

func (api *DesktopAPI) RebindShortcutByID(c *gin.Context) {
	if api.host == nil {
		apiError(c, http.StatusServiceUnavailable, "desktop host unavailable")
		return
	}
	contribID := c.Param("contributionId")
	var req struct {
		Accelerator string `json:"accelerator"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	if req.Accelerator == "" {
		apiError(c, http.StatusBadRequest, "accelerator is required")
		return
	}
	if err := api.host.RebindShortcut(contribID, req.Accelerator); err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"contributionId": contribID, "accelerator": req.Accelerator})
}

func (api *DesktopAPI) ListAllConflicts(c *gin.Context) {
	if api.host == nil {
		apiError(c, http.StatusServiceUnavailable, "desktop host unavailable")
		return
	}
	cr := api.host.GetConflicts()
	if cr == nil {
		c.JSON(http.StatusOK, gin.H{"items": []ConflictRecord{}, "total": 0})
		return
	}
	conflicts := cr.ListAll()
	c.JSON(http.StatusOK, gin.H{"items": conflicts, "total": len(conflicts)})
}

func (api *DesktopAPI) ResolveConflictByID(c *gin.Context) {
	if api.host == nil {
		apiError(c, http.StatusServiceUnavailable, "desktop host unavailable")
		return
	}
	conflictID := c.Param("conflictId")
	var req struct {
		Resolution string `json:"resolution"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	if req.Resolution == "" {
		apiError(c, http.StatusBadRequest, "resolution is required")
		return
	}
	cr := api.host.GetConflicts()
	if cr == nil {
		apiError(c, http.StatusNotFound, "conflict resolver unavailable")
		return
	}
	record, ok := cr.Get(conflictID)
	if !ok {
		apiError(c, http.StatusNotFound, fmt.Sprintf("conflict %s not found", conflictID))
		return
	}
	if record.Resolved {
		apiError(c, http.StatusBadRequest, fmt.Sprintf("conflict %s already resolved", conflictID))
		return
	}
	if err := cr.Resolve(conflictID, req.Resolution); err != nil {
		apiError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"conflictId": conflictID, "resolved": true})
}

func (api *DesktopAPI) GetCurrentSnapshot(c *gin.Context) {
	if api.host == nil {
		apiError(c, http.StatusServiceUnavailable, "desktop host unavailable")
		return
	}
	snapshot := api.host.GetSnapshot()
	if snapshot == nil {
		apiError(c, http.StatusNotFound, "no snapshot available")
		return
	}
	c.JSON(http.StatusOK, snapshot)
}

func (api *DesktopAPI) BuildNewSnapshot(c *gin.Context) {
	if api.host == nil {
		apiError(c, http.StatusServiceUnavailable, "desktop host unavailable")
		return
	}
	var req struct {
		HostReservedIDs map[string]bool `json:"hostReservedIds,omitempty"`
		UserPinnedOrder map[string]int  `json:"userPinnedOrder,omitempty"`
	}
	_ = c.ShouldBindJSON(&req)
	sortCtx := SortContext{
		HostReservedIDs: req.HostReservedIDs,
		UserPinnedOrder:  req.UserPinnedOrder,
	}
	snapshot := api.host.BuildSnapshot(sortCtx)
	c.JSON(http.StatusOK, snapshot)
}

func (api *DesktopAPI) ListAllContracts(c *gin.Context) {
	if api.host == nil {
		apiError(c, http.StatusServiceUnavailable, "desktop host unavailable")
		return
	}
	cr := api.host.GetContracts()
	if cr == nil {
		c.JSON(http.StatusOK, gin.H{"items": []DesktopContractDefinition{}, "total": 0})
		return
	}
	contracts := cr.ListContracts()
	c.JSON(http.StatusOK, gin.H{"items": contracts, "total": len(contracts)})
}

func (api *DesktopAPI) ListAllPermissions(c *gin.Context) {
	perms := GetDesktopPermissionDefs()
	c.JSON(http.StatusOK, gin.H{"items": perms, "total": len(perms)})
}

func (api *DesktopAPI) ListAllResources(c *gin.Context) {
	if api.host == nil {
		apiError(c, http.StatusServiceUnavailable, "desktop host unavailable")
		return
	}
	resources := api.host.ListResources()
	c.JSON(http.StatusOK, gin.H{"items": resources, "total": len(resources)})
}

func (api *DesktopAPI) CircuitStatus(c *gin.Context) {
	if api.host == nil {
		apiError(c, http.StatusServiceUnavailable, "desktop host unavailable")
		return
	}
	status := api.host.GetCircuitStatus()
	c.JSON(http.StatusOK, status)
}

func (api *DesktopAPI) CircuitReset(c *gin.Context) {
	if api.host == nil {
		apiError(c, http.StatusServiceUnavailable, "desktop host unavailable")
		return
	}
	api.host.ResetCircuit()
	c.JSON(http.StatusOK, gin.H{"reset": true})
}

func (h *DesktopHost) EnableContribution(contributionID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	resolved, exists := h.contributions[contributionID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrContributionNotFound, contributionID)
	}
	if resolved.Status == ContributionStatusDisabled {
		resolved.Status = ContributionStatusRegistered
	}
	return nil
}

func (h *DesktopHost) DisableContribution(contributionID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	resolved, exists := h.contributions[contributionID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrContributionNotFound, contributionID)
	}
	resolved.Status = ContributionStatusDisabled
	return nil
}

func (h *DesktopHost) RebindShortcut(contributionID, accelerator string) error {
	vr := ValidateAccelerator(accelerator)
	if !vr.Valid {
		if vr.IsReserved {
			return fmt.Errorf("%w: %s", ErrReservedShortcut, accelerator)
		}
		return fmt.Errorf("%w: %s", ErrInvalidAccelerator, vr.Reason)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	resolved, exists := h.contributions[contributionID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrContributionNotFound, contributionID)
	}
	if resolved.Definition.Shortcut == nil {
		return fmt.Errorf("%w: contribution has no shortcut", ErrInvalidShortcut)
	}
	oldNormalized := NormalizeAccelerator(resolved.Definition.Shortcut.Accelerator)
	if oldNormalized == vr.Normalized {
		return nil
	}
	if existingID, exists := h.shortcutsByAccel[vr.Normalized]; exists && existingID != contributionID {
		return fmt.Errorf("%w: accelerator %s already used by %s", ErrShortcutConflict, vr.Normalized, existingID)
	}
	delete(h.shortcutsByAccel, oldNormalized)
	resolved.Definition.Shortcut.Accelerator = accelerator
	h.shortcutsByAccel[vr.Normalized] = contributionID
	return nil
}

type CircuitStatusInfo struct {
	Open      bool  `json:"open"`
	Failures  int32 `json:"failures"`
	Threshold int32 `json:"threshold"`
}

func (h *DesktopHost) GetCircuitStatus() CircuitStatusInfo {
	return CircuitStatusInfo{
		Open:      h.circuitOpen.Load(),
		Failures:  h.circuitFailures.Load(),
		Threshold: h.circuitThreshold,
	}
}
