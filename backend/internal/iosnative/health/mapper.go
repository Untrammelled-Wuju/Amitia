package health

type HealthDataType struct {
	Identifier string
	Unit       string
	Category   string
}

var healthTypeRegistry = map[string]HealthDataType{
	"stepCount":               {Identifier: "HKQuantityTypeIdentifierStepCount", Unit: "count", Category: "quantity"},
	"distanceWalkingRunning":  {Identifier: "HKQuantityTypeIdentifierDistanceWalkingRunning", Unit: "m", Category: "quantity"},
	"activeEnergyBurned":      {Identifier: "HKQuantityTypeIdentifierActiveEnergyBurned", Unit: "kcal", Category: "quantity"},
	"basalEnergyBurned":       {Identifier: "HKQuantityTypeIdentifierBasalEnergyBurned", Unit: "kcal", Category: "quantity"},
	"heartRate":               {Identifier: "HKQuantityTypeIdentifierHeartRate", Unit: "count/min", Category: "quantity"},
	"restingHeartRate":        {Identifier: "HKQuantityTypeIdentifierRestingHeartRate", Unit: "count/min", Category: "quantity"},
	"walkingHeartRateAverage": {Identifier: "HKQuantityTypeIdentifierWalkingHeartRateAverage", Unit: "count/min", Category: "quantity"},
	"heartRateVariability":    {Identifier: "HKQuantityTypeIdentifierHeartRateVariabilitySDNN", Unit: "ms", Category: "quantity"},
	"oxygenSaturation":        {Identifier: "HKQuantityTypeIdentifierOxygenSaturation", Unit: "%", Category: "quantity"},
	"bodyMass":                {Identifier: "HKQuantityTypeIdentifierBodyMass", Unit: "kg", Category: "quantity"},
	"bodyMassIndex":           {Identifier: "HKQuantityTypeIdentifierBodyMassIndex", Unit: "count", Category: "quantity"},
	"bodyFatPercentage":       {Identifier: "HKQuantityTypeIdentifierBodyFatPercentage", Unit: "%", Category: "quantity"},
	"respiratoryRate":         {Identifier: "HKQuantityTypeIdentifierRespiratoryRate", Unit: "count/min", Category: "quantity"},
	"bodyTemperature":         {Identifier: "HKQuantityTypeIdentifierBodyTemperature", Unit: "degC", Category: "quantity"},
	"bloodPressure":           {Identifier: "HKQuantityTypeIdentifierBloodPressureSystolic", Unit: "mmHg", Category: "quantity"},
	"sleepAnalysis":           {Identifier: "HKCategoryTypeIdentifierSleepAnalysis", Unit: "", Category: "category"},
	"mindfulSession":          {Identifier: "HKCategoryTypeIdentifierMindfulSession", Unit: "", Category: "category"},
}

func ResolveHealthType(name string) (HealthDataType, bool) {
	dt, ok := healthTypeRegistry[name]
	return dt, ok
}

func SupportedHealthTypes() []string {
	types := make([]string, 0, len(healthTypeRegistry))
	for name := range healthTypeRegistry {
		types = append(types, name)
	}
	return types
}

var sleepStageMapping = map[string]string{
	"inBed":       "inBed",
	"awake":       "awake",
	"asleep":      "asleep",
	"core":        "core",
	"deep":        "deep",
	"REM":         "REM",
	"unspecified": "unspecified",
}

func ResolveSleepStage(code string) string {
	if stage, ok := sleepStageMapping[code]; ok {
		return stage
	}
	return "unspecified"
}
