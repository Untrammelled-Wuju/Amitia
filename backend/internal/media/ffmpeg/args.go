package ffmpeg

import "strings"

func BuildVersionArgs() []string {
	return []string{"-version"}
}

func BuildFFprobeArgs(localPath string) []string {
	return []string{
		"-v", "error",
		"-show_streams",
		"-show_format",
		"-print_format", "json",
		localPath,
	}
}

func BuildBaseFlags() []string {
	return []string{
		"-hide_banner",
		"-nostdin",
		"-loglevel", "error",
	}
}

func BuildProgressFlags() []string {
	return []string{
		"-progress", "pipe:1",
		"-nostats",
	}
}

func IsNetworkProtocol(input string) bool {
	lower := strings.ToLower(input)
	for _, proto := range []string{"http://", "https://", "tcp://", "udp://", "rtsp://", "rtmp://",
		"ftp://", "sftp://", "srt://", "rist://", "gopher://"} {
		if strings.HasPrefix(lower, proto) {
			return true
		}
	}
	return false
}

func IsAllowedProtocol(input string, allowed []string) bool {
	if len(allowed) == 0 {
		return !IsNetworkProtocol(input)
	}
	lower := strings.ToLower(input)
	for _, proto := range allowed {
		if strings.HasPrefix(lower, proto+"://") || lower == proto {
			return true
		}
	}
	for _, proto := range []string{"http://", "https://", "tcp://", "udp://", "rtsp://", "rtmp://",
		"ftp://", "sftp://", "srt://", "rist://", "gopher://"} {
		if strings.HasPrefix(lower, proto) {
			for _, a := range allowed {
				if a == proto[:len(proto)-3] {
					return true
				}
			}
			return false
		}
	}
	return true
}

func SanitizeInputPath(input string, config Config) error {
	if input == "" {
		return nil
	}
	if config.AllowNetworkProtocols {
		return nil
	}
	if IsNetworkProtocol(input) {
		return NewError(FFMPEG_PROTOCOL_FORBIDDEN, "network protocol not allowed: "+input)
	}
	if !IsAllowedProtocol(input, config.AllowedProtocols) {
		return NewError(FFMPEG_PROTOCOL_FORBIDDEN, "protocol not in allowlist: "+input)
	}
	return nil
}
