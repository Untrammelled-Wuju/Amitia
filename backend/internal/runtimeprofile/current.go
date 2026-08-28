package runtimeprofile

import (
	"os"
	"strings"
)

// CurrentProcessProfile resolves the profile of the already-started server
// process without consulting legacy application config. Server binaries launched
// by the desktop shell always pass --runtime-profile explicitly; environment is
// retained for service/container deployments.
func CurrentProcessProfile() Profile {
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if strings.HasPrefix(arg, cliFlag+"=") {
			if p, err := Parse(strings.TrimPrefix(arg, cliFlag+"=")); err == nil {
				return p
			}
		}
		if arg == cliFlag && i+1 < len(args) {
			if p, err := Parse(args[i+1]); err == nil {
				return p
			}
		}
	}
	if raw, ok := os.LookupEnv(envKey); ok {
		if p, err := Parse(raw); err == nil {
			return p
		}
	}
	return Default()
}
