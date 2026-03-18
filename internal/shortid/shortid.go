package shortid

// Format returns a truncated ID suitable for log messages.
// Docker container IDs are 64 hex chars; this returns the first 12.
func Format(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
