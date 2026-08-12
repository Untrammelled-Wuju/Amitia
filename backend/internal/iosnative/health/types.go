package health

import "time"

type SampleQuery struct {
	Type      string    `json:"type"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Limit     int       `json:"limit"`
	Ascending bool      `json:"ascending"`
}

type StatisticsQuery struct {
	Type      string    `json:"type"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"EndTime"`
	Statistic string    `json:"statistic"`
	Bucket    string    `json:"bucket"`
}

type WorkoutQuery struct {
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Limit     int       `json:"limit"`
}

type SleepQuery struct {
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Limit     int       `json:"limit"`
}

type ActivityQuery struct {
	Date time.Time `json:"date"`
}

type AuthorizationRequest struct {
	Types []string `json:"types"`
	Read  bool     `json:"read"`
	Write bool     `json:"write"`
}

type HealthSample struct {
	Type      string    `json:"type"`
	Value     float64   `json:"value"`
	Unit      string    `json:"unit"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Source    string    `json:"source"`
}

type HealthStatistics struct {
	Type      string    `json:"type"`
	Statistic string    `json:"statistic"`
	Value     float64   `json:"value"`
	Unit      string    `json:"unit"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Bucket    string    `json:"bucket,omitempty"`
}

type Workout struct {
	ID                string            `json:"id"`
	ActivityType      string            `json:"activityType"`
	StartTime         time.Time         `json:"startTime"`
	EndTime           time.Time         `json:"endTime"`
	Duration          float64           `json:"duration"`
	TotalEnergyBurned float64           `json:"totalEnergyBurned,omitempty"`
	TotalDistance     float64           `json:"totalDistance,omitempty"`
	Source            string            `json:"source"`
	HeartRateSummary  *HeartRateSummary `json:"heartRateSummary,omitempty"`
}

type HeartRateSummary struct {
	Average float64 `json:"average,omitempty"`
	Maximum float64 `json:"maximum,omitempty"`
	Minimum float64 `json:"minimum,omitempty"`
}

type SleepRecord struct {
	Stage     string    `json:"stage"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Duration  float64   `json:"duration"`
	Source    string    `json:"source"`
}

type ActivitySummary struct {
	ActiveEnergy float64 `json:"activeEnergy,omitempty"`
	ExerciseTime int     `json:"exerciseTime,omitempty"`
	StandHours   int     `json:"standHours,omitempty"`
	MoveGoal     float64 `json:"moveGoal,omitempty"`
	ExerciseGoal int     `json:"exerciseGoal,omitempty"`
	StandGoal    int     `json:"standGoal,omitempty"`
	Supported    bool    `json:"supported"`
}

type HealthProfile struct {
	DateOfBirth   *FieldAvailability `json:"dateOfBirth,omitempty"`
	BiologicalSex *FieldAvailability `json:"biologicalSex,omitempty"`
	BloodType     *FieldAvailability `json:"bloodType,omitempty"`
	WheelchairUse *FieldAvailability `json:"wheelchairUse,omitempty"`
}

type FieldAvailability struct {
	Available  bool   `json:"available"`
	Authorized bool   `json:"authorized"`
	Value      string `json:"value,omitempty"`
}

type AuthorizationStatus struct {
	HealthKitAvailable bool                         `json:"healthKitAvailable"`
	RequestStatus      string                       `json:"requestStatus"`
	Types              map[string]TypeAuthorization `json:"types"`
}

type TypeAuthorization struct {
	ReadRequested bool   `json:"readRequested"`
	ReadStatus    string `json:"readStatus"`
	WriteStatus   string `json:"writeStatus"`
}
