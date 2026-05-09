package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	OrderStatusCreated         = "created"
	OrderStatusPaying          = "paying"
	OrderStatusPaid            = "paid"
	OrderStatusCallbackPending = "callback_pending"
	OrderStatusCallbackSuccess = "callback_success"
	OrderStatusCallbackFailed  = "callback_failed"
)

type Order struct {
	OutTradeNo            string `json:"out_trade_no"`
	PID                   string `json:"pid"`
	PaymentType           string `json:"payment_type"`
	Subject               string `json:"subject"`
	Amount                string `json:"amount"`
	NotifyURL             string `json:"notify_url"`
	ReturnURL             string `json:"return_url"`
	AlipayTradeNo         string `json:"alipay_trade_no,omitempty"`
	Status                string `json:"status"`
	CallbackAttempts      int    `json:"callback_attempts"`
	LastCallbackError     string `json:"last_callback_error,omitempty"`
	CreatedAt             int64  `json:"created_at"`
	UpdatedAt             int64  `json:"updated_at"`
	PaidAt                int64  `json:"paid_at,omitempty"`
	CallbackSuccessAt     int64  `json:"callback_success_at,omitempty"`
	LastCallbackAttemptAt int64  `json:"last_callback_attempt_at,omitempty"`
}

func (o *Order) SameRequest(other *Order) bool {
	return o.OutTradeNo == other.OutTradeNo &&
		o.PID == other.PID &&
		o.PaymentType == other.PaymentType &&
		o.Subject == other.Subject &&
		o.Amount == other.Amount &&
		o.NotifyURL == other.NotifyURL &&
		o.ReturnURL == other.ReturnURL
}

type OrderStore interface {
	UpsertFromSubmit(order *Order) (*Order, bool, error)
	Find(outTradeNo string) (*Order, bool, error)
	MarkPaying(outTradeNo string) error
	MarkPaid(outTradeNo string, alipayTradeNo string) (*Order, bool, error)
	RecordCallback(outTradeNo string, success bool, errText string) error
	RetryableOrders(maxAttempts int) ([]*Order, error)
}

type FileOrderStore struct {
	mu     sync.Mutex
	path   string
	orders map[string]*Order
}

func NewFileOrderStore(path string) (*FileOrderStore, error) {
	store := &FileOrderStore{path: path, orders: map[string]*Order{}}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *FileOrderStore) UpsertFromSubmit(order *Order) (*Order, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	if existing, ok := s.orders[order.OutTradeNo]; ok {
		if !existing.SameRequest(order) {
			return nil, false, fmt.Errorf("order %s already exists with different fields", order.OutTradeNo)
		}
		return cloneOrder(existing), true, nil
	}
	order.Status = OrderStatusCreated
	order.CreatedAt = now
	order.UpdatedAt = now
	s.orders[order.OutTradeNo] = cloneOrder(order)
	if err := s.saveLocked(); err != nil {
		return nil, false, err
	}
	return cloneOrder(order), false, nil
}

func (s *FileOrderStore) Find(outTradeNo string) (*Order, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[outTradeNo]
	if !ok {
		return nil, false, nil
	}
	return cloneOrder(order), true, nil
}

func (s *FileOrderStore) MarkPaying(outTradeNo string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[outTradeNo]
	if !ok {
		return fmt.Errorf("order %s not found", outTradeNo)
	}
	if order.Status == OrderStatusCreated {
		order.Status = OrderStatusPaying
		order.UpdatedAt = time.Now().Unix()
		return s.saveLocked()
	}
	return nil
}

func (s *FileOrderStore) MarkPaid(outTradeNo string, alipayTradeNo string) (*Order, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[outTradeNo]
	if !ok {
		return nil, false, fmt.Errorf("order %s not found", outTradeNo)
	}
	if order.Status == OrderStatusCallbackSuccess {
		return cloneOrder(order), false, nil
	}
	wasNewPayment := order.PaidAt == 0
	now := time.Now().Unix()
	order.Status = OrderStatusCallbackPending
	order.AlipayTradeNo = alipayTradeNo
	if order.PaidAt == 0 {
		order.PaidAt = now
	}
	order.UpdatedAt = now
	if err := s.saveLocked(); err != nil {
		return nil, false, err
	}
	return cloneOrder(order), wasNewPayment, nil
}

func (s *FileOrderStore) RecordCallback(outTradeNo string, success bool, errText string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[outTradeNo]
	if !ok {
		return fmt.Errorf("order %s not found", outTradeNo)
	}
	now := time.Now().Unix()
	order.CallbackAttempts++
	order.LastCallbackAttemptAt = now
	order.UpdatedAt = now
	if success {
		order.Status = OrderStatusCallbackSuccess
		order.LastCallbackError = ""
		order.CallbackSuccessAt = now
	} else {
		order.Status = OrderStatusCallbackFailed
		order.LastCallbackError = errText
	}
	return s.saveLocked()
}

func (s *FileOrderStore) RetryableOrders(maxAttempts int) ([]*Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var orders []*Order
	for _, order := range s.orders {
		if (order.Status == OrderStatusCallbackPending || order.Status == OrderStatusCallbackFailed) &&
			order.CallbackAttempts < maxAttempts {
			orders = append(orders, cloneOrder(order))
		}
	}
	return orders, nil
}

func (s *FileOrderStore) load() error {
	if s.path == "" {
		return nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var orders map[string]*Order
	if err := common.Unmarshal(data, &orders); err != nil {
		return err
	}
	if orders != nil {
		s.orders = orders
	}
	return nil
}

func (s *FileOrderStore) saveLocked() error {
	if s.path == "" {
		return nil
	}
	data, err := common.Marshal(s.orders)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func cloneOrder(order *Order) *Order {
	if order == nil {
		return nil
	}
	cp := *order
	return &cp
}
