package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestReadPasswordFileDropsTheLineEnding is the file an editor or echo writes:
// the password, then a line ending that is not part of it.
func TestReadPasswordFileDropsTheLineEnding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "password")
	require.NoError(t, os.WriteFile(path, []byte("a demo password\r\n"), 0o600))

	password, err := readPasswordFile(path)
	require.NoError(t, err)
	require.Equal(t, "a demo password", password)
}

// TestReadPasswordFileRefusesAnEmptyFile is a file that holds no password,
// which must not turn into a demo with a new random one.
func TestReadPasswordFileRefusesAnEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "password")
	require.NoError(t, os.WriteFile(path, []byte("\n"), 0o600))

	_, err := readPasswordFile(path)
	require.Error(t, err)
}

// TestThePasswordFlagsCannotBothBeSet is two passwords for one account.
func TestThePasswordFlagsCannotBothBeSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "password")
	require.NoError(t, os.WriteFile(path, []byte("a demo password"), 0o600))

	require.Equal(t, exitUsage, run([]string{"-password", "another password", "-password-file", path}))
}
