package gitworktree

import (
	"os"
	"strings"
)

func cleanToolEnvironment() []string {
	source := os.Environ()
	result := make([]string, 0, len(source))
	for _, item := range source {
		key, _, found := strings.Cut(item, "=")
		if !found {
			continue
		}
		upper := strings.ToUpper(key)
		if strings.HasPrefix(upper, "GIT_") || upper == "SSH_ASKPASS" || upper == "GOFLAGS" || upper == "GO111MODULE" || upper == "GOENV" || upper == "GOWORK" || upper == "GOTOOLCHAIN" || upper == "GOPROXY" || upper == "GOSUMDB" {
			continue
		}
		result = append(result, item)
	}
	return result
}

func safeGitEnvironment() []string {
	return append(cleanToolEnvironment(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
	)
}

func safeCheckpointTestEnvironment() []string {
	allowed := map[string]bool{
		"PATH": true, "PATHEXT": true, "SYSTEMROOT": true, "WINDIR": true,
		"SYSTEMDRIVE": true, "COMSPEC": true, "HOME": true, "USERPROFILE": true,
		"HOMEDRIVE": true, "HOMEPATH": true, "APPDATA": true, "LOCALAPPDATA": true,
		"TMP": true, "TEMP": true, "TMPDIR": true, "TZ": true,
		"LANG": true, "LC_ALL": true, "LC_CTYPE": true,
		"GOROOT": true, "GOPATH": true, "GOCACHE": true,
	}
	env := make([]string, 0, len(allowed)+10)
	for _, item := range os.Environ() {
		key, _, ok := strings.Cut(item, "=")
		if ok && allowed[strings.ToUpper(key)] {
			env = append(env, item)
		}
	}
	return append(env,
		"GOFLAGS=",
		"GO111MODULE=on",
		"GOENV=off",
		"GOWORK=off",
		"GOTOOLCHAIN=local",
		"GOPROXY=off",
		"GOSUMDB=off",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
	)
}
