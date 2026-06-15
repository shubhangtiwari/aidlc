package util

func Identity(value string) string {
	return value
}

func StableKey(parts ...string) string {
	result := ""
	for _, part := range parts {
		result += part
	}
	return result
}
