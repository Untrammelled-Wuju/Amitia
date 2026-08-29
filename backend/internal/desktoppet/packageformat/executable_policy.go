package packageformat

import (
	"path"
	"strings"
)

// forbiddenExecutableExtensions is intentionally broader than native binaries.
// Desktop-pet packages are passive resource archives; they must never carry
// scripts, bytecode, installers, or platform application bundles that could be
// executed by a host process or by a user after extraction.
var forbiddenExecutableExtensions = map[string]struct{}{
	".appimage":   {},
	".apk":        {},
	".appx":       {},
	".appxbundle": {},
	".bat":        {},
	".bin":        {},
	".cmd":        {},
	".com":        {},
	".cpl":        {},
	".deb":        {},
	".dll":        {},
	".dmg":        {},
	".exe":        {},
	".gadget":     {},
	".hta":        {},
	".iso":        {},
	".jar":        {},
	".js":         {},
	".jse":        {},
	".lnk":        {},
	".mjs":        {},
	".msi":        {},
	".msix":       {},
	".msixbundle": {},
	".msp":        {},
	".mst":        {},
	".pif":        {},
	".pkg":        {},
	".ps1":        {},
	".py":         {},
	".pyc":        {},
	".rb":         {},
	".rpm":        {},
	".run":        {},
	".scr":        {},
	".sh":         {},
	".so":         {},
	".sys":        {},
	".vb":         {},
	".vbe":        {},
	".vbs":        {},
	".wasm":       {},
	".ws":         {},
	".wsf":        {},
	".wsh":        {},
}

var forbiddenExecutableBundleSuffixes = []string{
	".app",
	".bundle",
	".framework",
	".plugin",
}

func isForbiddenExecutable(packagePath string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(packagePath, "\\", "/"))
	for _, segment := range strings.Split(normalized, "/") {
		for _, suffix := range forbiddenExecutableBundleSuffixes {
			if strings.HasSuffix(segment, suffix) {
				return true
			}
		}
	}
	_, forbidden := forbiddenExecutableExtensions[strings.ToLower(path.Ext(normalized))]
	return forbidden
}
