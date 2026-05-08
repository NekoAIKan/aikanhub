package main

import "fmt"

func NewOrderStore(cfg Config) (OrderStore, error) {
	switch cfg.normalizedOrderStoreType() {
	case OrderStoreTypeFile:
		return NewFileOrderStore(cfg.OrderStorePath)
	case OrderStoreTypePostgres:
		return NewPostgresOrderStore(cfg.DatabaseURL)
	default:
		return nil, fmt.Errorf("unsupported ORDER_STORE_TYPE %q", cfg.OrderStoreType)
	}
}
