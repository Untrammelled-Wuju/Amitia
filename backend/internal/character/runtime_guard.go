package character

import (
	"sync"

	"gorm.io/gorm"
)

var runtimeSources sync.Map

func runtimeSource(db *gorm.DB) *sync.RWMutex {
	var key any = db
	if connection, err := db.DB(); err == nil {
		key = connection
	}
	value, _ := runtimeSources.LoadOrStore(key, &sync.RWMutex{})
	return value.(*sync.RWMutex)
}

func LockRoleSource(db *gorm.DB) func() { gate := runtimeSource(db); gate.Lock(); return gate.Unlock }

func LockRuntimeRole(db *gorm.DB) func() {
	gate := runtimeSource(db)
	gate.RLock()
	return gate.RUnlock
}
