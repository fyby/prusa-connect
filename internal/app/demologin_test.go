package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAHiddenLoginStaysOutOfTheBanner is a password read from a file: the
// banner names the file and prints neither the password nor a login line
// that carries it, while the login itself still signs in with it.
func TestAHiddenLoginStaysOutOfTheBanner(t *testing.T) {
	fleet := &demoFleet{login: demoLogin{username: demoUsername, password: "a demo password", hidden: true}}

	require.NotContains(t, fleet.browserLogin(), "a demo password")
	require.NotContains(t, fleet.curlLogin("http://127.0.0.1:8080"), "a demo password")
	require.Contains(t, fleet.passwordNote(), "-password-file")
	require.Contains(t, fleet.login.body(), "a demo password")
}

// TestAGeneratedLoginIsPrinted is the demo's own default: a password nobody
// chose is printed, because the banner is the only place it is told.
func TestAGeneratedLoginIsPrinted(t *testing.T) {
	fleet := &demoFleet{login: demoLogin{username: demoUsername, password: "generated"}}

	require.Contains(t, fleet.curlLogin("http://127.0.0.1:8080"), "generated")
}
