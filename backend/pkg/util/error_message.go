package util

func OriginalErrorMessage(err error, fallback string) string {
	if err != nil {
		return err.Error()
	}
	return fallback
}
