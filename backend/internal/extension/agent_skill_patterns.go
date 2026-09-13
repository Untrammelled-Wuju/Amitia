package extension

import "regexp"

var semverPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

var secretPattern = regexp.MustCompile(`(?i)(api[_-]?key|access[_-]?token|refresh[_-]?token|authorization|bearer|password|passwd|secret|private[_-]?key|client[_-]?secret|cookie|session|webhook[_-]?token)\s*[=:]\s*["']?[A-Za-z0-9_./+\-=]{8,}`)
