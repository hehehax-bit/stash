package manager

// SetInstanceForTest sets the package-level Manager instance returned by
// GetInstance. It is intended for tests that exercise code which calls
// GetInstance but want to avoid the full Initialize bootstrap path.
func SetInstanceForTest(m *Manager) {
	instance = m
}
