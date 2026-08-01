package v2

import (
	"gorm.io/gorm"
)

type Services struct {
	Sessions       SessionService
	Commands       CommandService
	Events         EventService
	ActualStates   ActualStateService
}

func NewServices(db *gorm.DB) *Services {
	return &Services{
		Sessions:     NewSessionService(db),
		Commands:     NewCommandService(db),
		Events:       NewEventService(db),
		ActualStates: NewActualStateService(db),
	}
}
