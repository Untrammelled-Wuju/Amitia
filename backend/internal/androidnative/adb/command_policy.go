package adb

import (
	"strings"
	"unicode"
)

type CommandPolicy struct {
	AllowedExecutables map[string]CommandRule
}

type CommandRule struct {
	AllowNoArgs      bool
	AllowedArgs      []string
	ArgValidator     func(args []string) error
	MaxArgs          int
}

func NewCommandPolicy() *CommandPolicy {
	return &CommandPolicy{
		AllowedExecutables: map[string]CommandRule{
			"getprop": {
				AllowNoArgs: true,
				AllowedArgs: []string{},
				ArgValidator: func(args []string) error {
					if len(args) > 1 {
						return &PolicyError{Code: ADB_INVALID_ARGUMENT, Message: "getprop accepts at most one argument"}
					}
					for _, arg := range args {
						if !isValidPropertyName(arg) {
							return &PolicyError{Code: ADB_INVALID_ARGUMENT, Message: "invalid property name: " + arg}
						}
					}
					return nil
				},
				MaxArgs: 1,
			},
			"id": {
				AllowNoArgs: true,
				MaxArgs:     0,
			},
			"uname": {
				AllowNoArgs: true,
				AllowedArgs: []string{"-a", "-m", "-r", "-s"},
				ArgValidator: func(args []string) error {
					for _, arg := range args {
						if !isValidUnameArg(arg) {
							return &PolicyError{Code: ADB_INVALID_ARGUMENT, Message: "invalid uname flag: " + arg}
						}
					}
					return nil
				},
				MaxArgs: 1,
			},
		},
	}
}

func (p *CommandPolicy) Validate(executable string, args []string) error {
	rule, ok := p.AllowedExecutables[executable]
	if !ok {
		return &PolicyError{Code: ADB_COMMAND_NOT_ALLOWED, Message: "command not allowed: " + executable}
	}

	if len(args) > rule.MaxArgs {
		return &PolicyError{Code: ADB_INVALID_ARGUMENT, Message: "too many arguments for " + executable}
	}

	if len(args) == 0 && !rule.AllowNoArgs {
		return &PolicyError{Code: ADB_INVALID_ARGUMENT, Message: executable + " requires arguments"}
	}

	if rule.ArgValidator != nil {
		if err := rule.ArgValidator(args); err != nil {
			return err
		}
	}

	return nil
}

func (p *CommandPolicy) IsAllowed(executable string) bool {
	_, ok := p.AllowedExecutables[executable]
	return ok
}

func isValidPropertyName(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

func isValidUnameArg(s string) bool {
	for _, allowed := range []string{"-a", "-m", "-r", "-s"} {
		if s == allowed {
			return true
		}
	}
	return false
}

type PolicyError struct {
	Code    string
	Message string
}

func (e *PolicyError) Error() string {
	return e.Code + ": " + e.Message
}

func isShellExecutable(executable string) bool {
	shellNames := []string{"sh", "bash", "zsh", "toybox", "dash", "busybox"}
	for _, name := range shellNames {
		if strings.EqualFold(executable, name) {
			return true
		}
	}
	return false
}
