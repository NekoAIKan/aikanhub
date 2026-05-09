package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewHTTPServerSetsDefensiveTimeouts(t *testing.T) {
	cfg := validTestConfig(t)
	server := newHTTPServer(cfg, http.NotFoundHandler())

	require.Equal(t, cfg.ListenAddr, server.Addr)
	require.Equal(t, defaultReadHeaderTimeout, server.ReadHeaderTimeout)
	require.Equal(t, defaultReadTimeout, server.ReadTimeout)
	require.Equal(t, defaultWriteTimeout, server.WriteTimeout)
	require.Equal(t, defaultIdleTimeout, server.IdleTimeout)
	require.Equal(t, 16*1024, server.MaxHeaderBytes)
}
