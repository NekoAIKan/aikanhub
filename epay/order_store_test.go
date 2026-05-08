package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewOrderStoreSelectsFileStoreByDefault(t *testing.T) {
	cfg := validTestConfig(t)
	cfg.OrderStoreType = ""

	store, err := NewOrderStore(cfg)

	require.NoError(t, err)
	require.IsType(t, &FileOrderStore{}, store)
}

func TestNewOrderStoreRejectsUnknownStoreType(t *testing.T) {
	cfg := validTestConfig(t)
	cfg.OrderStoreType = "redis"

	_, err := NewOrderStore(cfg)

	require.ErrorContains(t, err, "unsupported ORDER_STORE_TYPE")
}
