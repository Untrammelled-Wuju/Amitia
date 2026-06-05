package companion



type SleepSetting struct {
	ID        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	BedTime   string    `gorm:"column:bed_time;default:23:00" json:"bedTime"`
	WakeTime  string    `gorm:"column:wake_time;default:07:00" json:"wakeTime"`
	Enabled   int       `gorm:"column:enabled;default:1" json:"enabled"`
	CreatedAt string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt string `gorm:"column:updated_at" json:"updatedAt"`
}

func (SleepSetting) TableName() string { return "sleep_settings" }

type FixedEvent struct {
	ID          int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"column:name;not null" json:"name"`
	Description string    `gorm:"column:description" json:"description"`
	WeekDay     int       `gorm:"column:week_day;default:-1" json:"weekDay"`
	StartTime   string    `gorm:"column:start_time" json:"startTime"`
	EndTime     string    `gorm:"column:end_time" json:"endTime"`
	EventType   string    `gorm:"column:event_type;default:custom" json:"eventType"`
	Location    string    `gorm:"column:location" json:"location"`
	Enabled     int       `gorm:"column:enabled;default:1" json:"enabled"`
	CreatedAt   string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   string `gorm:"column:updated_at" json:"updatedAt"`
}

func (FixedEvent) TableName() string { return "fixed_events" }

type SpecialEvent struct {
	ID          int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"column:name;not null" json:"name"`
	Description string    `gorm:"column:description" json:"description"`
	EventDate   string    `gorm:"column:event_date" json:"eventDate"`
	StartTime   string    `gorm:"column:start_time" json:"startTime"`
	EndTime     string    `gorm:"column:end_time" json:"endTime"`
	EventType   string    `gorm:"column:event_type;default:custom" json:"eventType"`
	Location    string    `gorm:"column:location" json:"location"`
	Enabled     int       `gorm:"column:enabled;default:1" json:"enabled"`
	CreatedAt   string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   string `gorm:"column:updated_at" json:"updatedAt"`
}

func (SpecialEvent) TableName() string { return "special_events" }

type ClassAdjustment struct {
	ID          int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Date        string    `gorm:"column:date" json:"date"`
	SlotIndex   int       `gorm:"column:slot_index" json:"slotIndex"`
	ClassName   string    `gorm:"column:class_name" json:"className"`
	AdjustType  string    `gorm:"column:adjust_type;default:swap" json:"adjustType"`
	Description string    `gorm:"column:description" json:"description"`
	CreatedAt   string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   string `gorm:"column:updated_at" json:"updatedAt"`
}

func (ClassAdjustment) TableName() string { return "class_adjustments" }

type LifestyleTendency struct {
	ID         int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Activity   string    `gorm:"column:activity" json:"activity"`
	Intensity  int       `gorm:"column:intensity;default:50" json:"intensity"`
	Schedule   string    `gorm:"column:schedule" json:"schedule"`
	Preference string    `gorm:"column:preference" json:"preference"`
	UpdatedAt  string `gorm:"column:updated_at" json:"updatedAt"`
}

func (LifestyleTendency) TableName() string { return "lifestyle_tendencies" }

type WorkProfile struct {
	ID          int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	JobTitle    string    `gorm:"column:job_title" json:"jobTitle"`
	WorkHours   string    `gorm:"column:work_hours" json:"workHours"`
	WorkDays    string    `gorm:"column:work_days" json:"workDays"`
	Description string    `gorm:"column:description" json:"description"`
	UpdatedAt   string `gorm:"column:updated_at" json:"updatedAt"`
}

func (WorkProfile) TableName() string { return "work_profiles" }

type ActiveMessageSetting struct {
	Enabled     int    `gorm:"column:enabled;default:1" json:"enabled"`
	MinInterval int    `gorm:"column:min_interval;default:60" json:"minInterval"`
	MaxPerDay   int    `gorm:"column:max_per_day;default:6" json:"maxPerDay"`
	Channel     string `gorm:"column:channel;default:all" json:"channel"`
}

type ScheduleSlot struct {
	DayOfWeek int    `json:"dayOfWeek"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Name      string `json:"name"`
	Type      string `json:"type"`
}
