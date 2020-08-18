package reconciliation

var (
	// Provides reasonable defaults for the logger container.
	DefaultsLoggerContainer = buildResourceRequirements(100, 64)

	// Provides reasonable defaults for the configuration container.
	DefaultsConfigInitContainer = buildResourceRequirements(256, 256)

	// Provides reasonable defaults for the reaper sidecar container.
	DefaultsReaperContainer = buildResourceRequirements(2000, 512)
)
