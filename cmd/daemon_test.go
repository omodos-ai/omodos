package cmd

import "testing"

func TestDaemonDeprecated(t *testing.T) {
	// The standalone daemon has been deprecated.
	// IPC and pipe testing is now handled by pipe_test.go in TestUnifiedPipesAndBroadcasting.
}
