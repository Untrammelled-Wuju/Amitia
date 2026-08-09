package conformance

type Validator interface {
	Name() string
	Validate(data []byte) error
}

type Case struct {
	Name        string
	Description string
	Input       []byte
	ExpectedValid bool
	Validator   Validator
}

func NewCase(name, description string, input []byte, expectedValid bool, validator Validator) Case {
	return Case{
		Name:          name,
		Description:     description,
		Input:         input,
		ExpectedValid: expectedValid,
		Validator:     validator,
	}
}
