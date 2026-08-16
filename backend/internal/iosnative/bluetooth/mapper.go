package bluetooth

import "github.com/u-ai/backend/internal/nativebridge"

func AuthorizationStatusFromNative(native string) AuthorizationStatus {
	switch native {
	case "allowedAlways":
		return AuthAllowed
	case "denied":
		return AuthDenied
	case "restricted":
		return AuthRestricted
	default:
		return AuthNotDetermined
	}
}

func CentralStateFromNative(native string) CentralState {
	switch native {
	case "unknown":
		return StateUnknown
	case "resetting":
		return StateResetting
	case "unsupported":
		return StateUnsupported
	case "unauthorized":
		return StateUnauthorized
	case "poweredOff":
		return StatePoweredOff
	case "poweredOn":
		return StatePoweredOn
	default:
		return StateUnknown
	}
}

func MapErrorToNativeBridge(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	return ErrOutcomeUnknown, err.Error()
}

func MapCodeToMessage(code string) string {
	switch code {
	case ErrBluetoothUnsupported:
		return "bluetooth is not supported on this device"
	case ErrBluetoothUnauthorized:
		return "bluetooth authorization not determined"
	case ErrBluetoothDenied:
		return "bluetooth access denied"
	case ErrBluetoothRestricted:
		return "bluetooth access restricted"
	case ErrBluetoothPoweredOff:
		return "bluetooth is powered off"
	case ErrPeripheralNotFound:
		return "peripheral not found"
	case ErrConnectionFailed:
		return "connection failed"
	case ErrConnectionTimeout:
		return "connection timed out"
	case ErrServiceNotFound:
		return "service not found"
	case ErrCharacteristicNotFound:
		return "characteristic not found"
	case ErrDescriptorNotFound:
		return "descriptor not found"
	case ErrCharacteristicWriteNotAllowed:
		return "characteristic does not support write"
	case ErrCharacteristicReadNotAllowed:
		return "characteristic does not support read"
	case ErrWriteTooLong:
		return "write value exceeds maximum length"
	case ErrSubscribeNotSupported:
		return "characteristic does not support subscribe"
	case ErrNativeBridgeUnavailable:
		return "ios native bridge is not available"
	default:
		return code
	}
}

func MapCharacteristicProperty(prop string) string {
	switch prop {
	case "broadcast":
		return "broadcast"
	case "read":
		return "read"
	case "writeWithoutResponse":
		return "write_without_response"
	case "write":
		return "write"
	case "notify":
		return "notify"
	case "indicate":
		return "indicate"
	case "authenticatedSignedWrites":
		return "authenticated_signed_writes"
	case "extendedProperties":
		return "extended_properties"
	case "notifyEncryptionRequired":
		return "notify_encryption_required"
	case "indicateEncryptionRequired":
		return "indicate_encryption_required"
	default:
		return prop
	}
}

func HasProperty(props []string, target string) bool {
	for _, p := range props {
		if p == target {
			return true
		}
	}
	return false
}

func NewBluetoothError(request nativebridge.Request, code, message string) nativebridge.Response {
	return nativebridge.Response{
		ProtocolVersion: request.ProtocolVersion,
		RequestId:       request.RequestId,
		Status:          "error",
		Error: &nativebridge.Error{
			Code:    code,
			Message: message,
		},
	}
}
