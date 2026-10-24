package chat

func conservativeModelOutputCeiling(configured, legacy int, anthropic bool, thinking int) int64 {
	output := configured
	if legacy > output {
		output = legacy
	}
	if output <= 0 {
		output = 4096
	}
	if anthropic && thinking > 0 && output <= thinking {
		output = thinking + 2048
	}
	return int64(output)
}
