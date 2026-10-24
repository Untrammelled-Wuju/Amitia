package mindruntime

func CeilingOwnedModelCost(input, output, inputRate, outputRate int64) (int64, bool) {
	if input < 0 || output < 0 || input > 100000000 || output > 100000000 || inputRate < 1 || inputRate > 100000000 || outputRate < 1 || outputRate > 100000000 {
		return 0, false
	}
	total := input*inputRate + output*outputRate
	return (total + 999999) / 1000000, true
}
