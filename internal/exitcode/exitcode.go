package exitcode

// Standard semantic exit codes for the agy-sync CLI.
const (
	// Success indicates that the command completed without error.
	Success = 0

	// GeneralError indicates an unspecified runtime failure.
	GeneralError = 1

	// UsageError indicates invalid flags or command arguments.
	UsageError = 2

	// ConfigError indicates a missing or invalid configuration.
	ConfigError = 3

	// DaemonError indicates a failure related to daemon lifecycle management.
	DaemonError = 4
)
