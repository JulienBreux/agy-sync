package exitcode_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/julienbreux/agy-sync/internal/exitcode"
)

func TestExitCodes(t *testing.T) {
	assert.Equal(t, 0, exitcode.Success)
	assert.Equal(t, 1, exitcode.GeneralError)
	assert.Equal(t, 2, exitcode.UsageError)
	assert.Equal(t, 3, exitcode.ConfigError)
	assert.Equal(t, 4, exitcode.DaemonError)
}
