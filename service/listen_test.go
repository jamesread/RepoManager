package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListenAddrDefault(t *testing.T) {
	t.Setenv("PORT", "")
	require.Equal(t, ":8080", ListenAddr())
}

func TestListenAddrBarePort(t *testing.T) {
	t.Setenv("PORT", "9090")
	require.Equal(t, ":9090", ListenAddr())
}

func TestListenAddrFullAddress(t *testing.T) {
	t.Setenv("PORT", "0.0.0.0:9090")
	require.Equal(t, "0.0.0.0:9090", ListenAddr())
}
