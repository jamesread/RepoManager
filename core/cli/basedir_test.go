package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBasedirEmpty(t *testing.T) {
	basedir, err := ParseBasedir(nil)
	require.NoError(t, err)
	require.Equal(t, "", basedir)

	basedir, err = ParseBasedir([]string{"/tmp/projects"})
	require.NoError(t, err)
	require.Equal(t, "/tmp/projects", basedir)
}

func TestParseBasedirFlag(t *testing.T) {
	basedir, err := ParseBasedir([]string{"--basedir", "~/sandbox/"})
	require.NoError(t, err)
	require.Equal(t, "~/sandbox/", basedir)

	basedir, err = ParseBasedir([]string{"--basedir=~/sandbox/"})
	require.NoError(t, err)
	require.Equal(t, "~/sandbox/", basedir)
}

func TestParseBasedirErrors(t *testing.T) {
	_, err := ParseBasedir([]string{"--basedir"})
	require.Error(t, err)

	_, err = ParseBasedir([]string{"--basedir", "/a", "/b"})
	require.Error(t, err)

	_, err = ParseBasedir([]string{"--unknown"})
	require.Error(t, err)
}
