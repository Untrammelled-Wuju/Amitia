package config

const (
	ErrInvalidType      = "INVALID_TYPE"
	ErrRequiredField    = "REQUIRED_FIELD"
	ErrValueNotInEnum   = "VALUE_NOT_IN_ENUM"
	ErrNumberOutOfRange = "NUMBER_OUT_OF_RANGE"
	ErrStringLength     = "STRING_LENGTH"
	ErrInvalidJSON      = "INVALID_JSON"
	ErrSecretRefInvalid = "SECRET_REF_INVALID"
	ErrProviderUnknown  = "PROVIDER_UNKNOWN"
	ErrSchemaNotFound   = "SCHEMA_NOT_FOUND"
	ErrConfigNotFound   = "CONFIG_NOT_FOUND"
	ErrInvalidScope     = "INVALID_SCOPE"
	ErrCorruptConfig    = "CORRUPT_CONFIG"
	ErrPathViolation    = "PATH_VIOLATION"
)

type ValidationError struct {
	Key     string `json:"key"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return e.Key + ": " + e.Message
}

type ValidationErrorList []ValidationError

func (l ValidationErrorList) Error() string {
	if len(l) == 0 {
		return ""
	}
	return l[0].Error()
}

func (l ValidationErrorList) HasErrors() bool {
	return len(l) > 0
}

type ConfigError struct {
	Op   string
	Kind string
	ID   string
	Cause error
}

func (e ConfigError) Error() string {
	if e.Cause != nil {
		return "config." + e.Op + ": " + e.Cause.Error()
	}
	return "config." + e.Op + ": " + e.Kind
}

func newConfigError(op, kind, id string, cause error) ConfigError {
	return ConfigError{Op: op, Kind: kind, ID: id, Cause: cause}
}

func isConfigError(err error, kind string) bool {
	if err == nil {
		return false
	}
	if ce, ok := err.(ConfigError); ok {
		return ce.Kind == kind
	}
	return false
}
