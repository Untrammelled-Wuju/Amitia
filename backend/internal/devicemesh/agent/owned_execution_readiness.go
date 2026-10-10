package agent

import (
	"database/sql"
	"errors"
	"reflect"
	"strings"
)

func (h *LocalHandler) ValidateJournalDatabaseWiring(db *sql.DB) error {
	if h == nil {
		return errors.New("本机设备 Agent 尚未初始化")
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.executionJournal.ValidateDatabaseWiring(db)
}

func (h *LocalHandler) ValidateOwnedExecutionWiring() error {
	if h == nil {
		return errors.New("本机设备 Agent 尚未初始化")
	}
	h.providerMu.Lock()
	providerMissing := ownedDispatcherMissing(h.providerContext)
	h.providerMu.Unlock()
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.identity == nil || h.credStore == nil || h.executionGuard == nil || h.executionJournal == nil || strings.TrimSpace(h.localCoreID) == "" || providerMissing {
		return errors.New("本机身份、执行守卫或持久执行记录未接通")
	}
	if ownedDispatcherMissing(h.dispatcher) {
		return errors.New("本机数据或事件路由未初始化")
	}
	if err := h.executionJournal.ValidateWiring(); err != nil {
		return err
	}
	dispatcher, ok := h.dispatcher.(RuntimeContextDispatcher)
	if !ok || dispatcher.ResolveContext("coordination.data") == nil || dispatcher.ResolveContext("task.host.source-event") == nil {
		return errors.New("本机数据或事件路由不能保留原执行上下文")
	}
	return nil
}

func ownedDispatcherMissing(dispatcher any) bool {
	if dispatcher == nil {
		return true
	}
	value := reflect.ValueOf(dispatcher)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	}
	return false
}

func (h *LocalHandler) ValidateOwnerWiring(coreID, sourceID string) error {
	if h == nil || coreID == "" || sourceID == "" {
		return errors.New("Core 与原设备身份未接通")
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.localCoreID != coreID || h.identity == nil {
		return errors.New("角色归属 Core 与本机 Core 身份不一致")
	}
	identity, err := h.identity.Load()
	if err != nil {
		return err
	}
	if identity == nil || identity.DeviceID.String() != sourceID {
		return errors.New("设备数据端口与本机持久身份不一致")
	}
	return nil
}
