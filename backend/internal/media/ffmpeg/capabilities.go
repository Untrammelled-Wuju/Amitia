package ffmpeg

type CapabilityState struct {
	Supported bool `json:"supported"`

	FFmpegAvailable  bool `json:"ffmpegAvailable"`
	FFprobeAvailable bool `json:"ffprobeAvailable"`

	FFmpegVersion  string `json:"ffmpegVersion,omitempty"`
	FFprobeVersion string `json:"ffprobeVersion,omitempty"`

	RuntimeID string `json:"runtimeId,omitempty"`

	Platform     string `json:"platform,omitempty"`
	Architecture string `json:"architecture,omitempty"`

	Source string `json:"source,omitempty"`

	NetworkProtocolsAllowed bool `json:"networkProtocolsAllowed"`

	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

type Capabilities struct {
	Available bool

	Probe       bool
	StreamCopy  bool

	CommonDemuxers []string
	CommonMuxers   []string
	CommonDecoders []string
	CommonEncoders []string

	HardwareAcceleration []string
}

func DisabledCapabilityState(runtimeID, reason string) CapabilityState {
	return CapabilityState{
		Supported:               false,
		FFmpegAvailable:         false,
		FFprobeAvailable:        false,
		RuntimeID:               runtimeID,
		NetworkProtocolsAllowed: false,
		State:                   "disabled",
		Reason:                  reason,
	}
}

func (c CapabilityState) IsUsable() bool {
	return c.Supported && c.FFmpegAvailable
}
