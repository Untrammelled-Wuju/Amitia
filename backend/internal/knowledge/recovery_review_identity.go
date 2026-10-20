package knowledge

import (
	"encoding/hex"
	"strings"

	"github.com/u-ai/backend/internal/auth"
)

const recoveryPrincipalV2Prefix = "device-v2:"

func recoveryDevicePrincipalHash(actor *auth.ActorContext) (string, bool) {
	if actor == nil || !actor.HasPermission(auth.PermSystemAdmin) {
		return "", false
	}
	if actor.PrincipalType != auth.PrincipalTrustedDevice && (actor.PrincipalType != auth.PrincipalLocalUI || !actor.IsLocalTrusted) {
		return "", false
	}
	space := strings.TrimSpace(actor.SpaceID.String())
	device := strings.TrimSpace(actor.DeviceID.String())
	if space == "" || device == "" {
		return "", false
	}
	return recoveryPrincipalV2Prefix + hashAuditBytes([]byte("knowledge-recovery-principal-v2\x00"+space+"\x00"+device)), true
}

func recoveryPrincipalIsVerified(actorHash string) bool {
	if !strings.HasPrefix(actorHash, recoveryPrincipalV2Prefix) || len(actorHash) != len(recoveryPrincipalV2Prefix)+64 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(actorHash, recoveryPrincipalV2Prefix))
	return err == nil
}
