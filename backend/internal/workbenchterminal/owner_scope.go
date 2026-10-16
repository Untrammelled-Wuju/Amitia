package workbenchterminal

import (
	"strconv"
	"strings"
)

func terminalOwnerKey(space, device, runtime, session, conversation string) string {
	fields := [...]string{space, device, runtime, session, conversation}
	var key strings.Builder
	for _, field := range fields {
		key.WriteString(strconv.Itoa(len(field)))
		key.WriteByte(':')
		key.WriteString(field)
	}
	return key.String()
}
