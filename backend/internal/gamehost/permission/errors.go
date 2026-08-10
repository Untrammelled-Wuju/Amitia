package permission

import "fmt"

var (
	ErrInvalidSubject  = fmt.Errorf("gamehost: permission: invalid subject")
	ErrRuntimeInactive = fmt.Errorf("gamehost: permission: runtime not active")
)
