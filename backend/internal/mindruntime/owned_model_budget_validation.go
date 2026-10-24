package mindruntime

func trustworthyOwnedTokenUsage(reported bool, input, output, cached, total int64) bool {
	return reported && total > 0 && input >= 0 && output >= 0 && cached >= 0 && total >= input && total-input >= output
}
