package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type PostgresOrderStore struct {
	db *gorm.DB
}

type postgresOrder struct {
	OutTradeNo            string `gorm:"primaryKey;column:out_trade_no;type:text"`
	PID                   string `gorm:"column:pid;type:text;not null"`
	PaymentType           string `gorm:"column:payment_type;type:text;not null"`
	Subject               string `gorm:"column:subject;type:text;not null"`
	Amount                string `gorm:"column:amount;type:text;not null"`
	NotifyURL             string `gorm:"column:notify_url;type:text;not null"`
	ReturnURL             string `gorm:"column:return_url;type:text;not null"`
	AlipayTradeNo         string `gorm:"column:alipay_trade_no;type:text;index"`
	Status                string `gorm:"column:status;type:text;not null;index"`
	CallbackAttempts      int    `gorm:"column:callback_attempts;not null;default:0"`
	LastCallbackError     string `gorm:"column:last_callback_error;type:text"`
	CreatedAt             int64  `gorm:"column:created_at;not null;autoCreateTime:false"`
	UpdatedAt             int64  `gorm:"column:updated_at;not null;autoUpdateTime:false;index"`
	PaidAt                int64  `gorm:"column:paid_at;not null;default:0"`
	CallbackSuccessAt     int64  `gorm:"column:callback_success_at;not null;default:0"`
	LastCallbackAttemptAt int64  `gorm:"column:last_callback_attempt_at;not null;default:0"`
}

func (postgresOrder) TableName() string {
	return "epay_orders"
}

type postgresOrderEvent struct {
	ID         uint64 `gorm:"primaryKey;column:id;autoIncrement"`
	OutTradeNo string `gorm:"column:out_trade_no;type:text;not null;index"`
	EventType  string `gorm:"column:event_type;type:text;not null;index"`
	Payload    string `gorm:"column:payload;type:text;not null"`
	CreatedAt  int64  `gorm:"column:created_at;not null;autoCreateTime:false;index"`
}

func (postgresOrderEvent) TableName() string {
	return "epay_order_events"
}

func NewPostgresOrderStore(databaseURL string) (*PostgresOrderStore, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres order store: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(5)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	if err := db.AutoMigrate(&postgresOrder{}, &postgresOrderEvent{}); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("migrate postgres order store: %w", err)
	}
	return &PostgresOrderStore{db: db}, nil
}

func (s *PostgresOrderStore) UpsertFromSubmit(order *Order) (*Order, bool, error) {
	now := time.Now().Unix()
	row := postgresOrderFromOrder(order)
	row.Status = OrderStatusCreated
	row.CreatedAt = now
	row.UpdatedAt = now

	existed := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 1 {
			return recordOrderEvent(tx, row.OutTradeNo, "submit_created", order)
		}
		existed = true

		var existing postgresOrder
		if err := tx.First(&existing, "out_trade_no = ?", order.OutTradeNo).Error; err != nil {
			return err
		}
		existingOrder := existing.toOrder()
		if !existingOrder.SameRequest(order) {
			return fmt.Errorf("order %s already exists with different fields", order.OutTradeNo)
		}
		*order = *existingOrder
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	stored, ok, err := s.Find(order.OutTradeNo)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, fmt.Errorf("order %s not found after upsert", order.OutTradeNo)
	}
	return stored, existed, nil
}

func (s *PostgresOrderStore) Find(outTradeNo string) (*Order, bool, error) {
	var row postgresOrder
	err := s.db.First(&row, "out_trade_no = ?", outTradeNo).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return row.toOrder(), true, nil
}

func (s *PostgresOrderStore) MarkPaying(outTradeNo string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		row, err := lockPostgresOrder(tx, outTradeNo)
		if err != nil {
			return err
		}
		if row.Status != OrderStatusCreated {
			return nil
		}
		row.Status = OrderStatusPaying
		row.UpdatedAt = time.Now().Unix()
		if err := tx.Save(row).Error; err != nil {
			return err
		}
		return recordOrderEvent(tx, outTradeNo, "mark_paying", row.toOrder())
	})
}

func (s *PostgresOrderStore) MarkPaid(outTradeNo string, alipayTradeNo string) (*Order, bool, error) {
	var paidOrder *Order
	var shouldCallback bool
	err := s.db.Transaction(func(tx *gorm.DB) error {
		row, err := lockPostgresOrder(tx, outTradeNo)
		if err != nil {
			return err
		}
		if row.Status == OrderStatusCallbackSuccess {
			paidOrder = row.toOrder()
			return nil
		}
		now := time.Now().Unix()
		shouldCallback = row.PaidAt == 0
		row.Status = OrderStatusCallbackPending
		row.AlipayTradeNo = alipayTradeNo
		if row.PaidAt == 0 {
			row.PaidAt = now
		}
		row.UpdatedAt = now
		if err := tx.Save(row).Error; err != nil {
			return err
		}
		paidOrder = row.toOrder()
		if shouldCallback {
			return recordOrderEvent(tx, outTradeNo, "alipay_paid", paidOrder)
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return cloneOrder(paidOrder), shouldCallback, nil
}

func (s *PostgresOrderStore) RecordCallback(outTradeNo string, success bool, errText string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		row, err := lockPostgresOrder(tx, outTradeNo)
		if err != nil {
			return err
		}
		now := time.Now().Unix()
		row.CallbackAttempts++
		row.LastCallbackAttemptAt = now
		row.UpdatedAt = now
		if success {
			row.Status = OrderStatusCallbackSuccess
			row.LastCallbackError = ""
			row.CallbackSuccessAt = now
		} else {
			row.Status = OrderStatusCallbackFailed
			row.LastCallbackError = errText
		}
		if err := tx.Save(row).Error; err != nil {
			return err
		}
		eventType := "callback_failed"
		if success {
			eventType = "callback_success"
		}
		return recordOrderEvent(tx, outTradeNo, eventType, row.toOrder())
	})
}

func (s *PostgresOrderStore) RetryableOrders(maxAttempts int) ([]*Order, error) {
	var rows []postgresOrder
	err := s.db.
		Where("status IN ? AND callback_attempts < ?", []string{OrderStatusCallbackPending, OrderStatusCallbackFailed}, maxAttempts).
		Order("updated_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	orders := make([]*Order, 0, len(rows))
	for _, row := range rows {
		orders = append(orders, row.toOrder())
	}
	return orders, nil
}

func lockPostgresOrder(tx *gorm.DB, outTradeNo string) (*postgresOrder, error) {
	var row postgresOrder
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "out_trade_no = ?", outTradeNo).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("order %s not found", outTradeNo)
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func recordOrderEvent(tx *gorm.DB, outTradeNo string, eventType string, payload any) error {
	payloadBytes, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	event := &postgresOrderEvent{
		OutTradeNo: outTradeNo,
		EventType:  eventType,
		Payload:    string(payloadBytes),
		CreatedAt:  time.Now().Unix(),
	}
	return tx.Create(event).Error
}

func postgresOrderFromOrder(order *Order) *postgresOrder {
	return &postgresOrder{
		OutTradeNo:            order.OutTradeNo,
		PID:                   order.PID,
		PaymentType:           order.PaymentType,
		Subject:               order.Subject,
		Amount:                order.Amount,
		NotifyURL:             order.NotifyURL,
		ReturnURL:             order.ReturnURL,
		AlipayTradeNo:         order.AlipayTradeNo,
		Status:                order.Status,
		CallbackAttempts:      order.CallbackAttempts,
		LastCallbackError:     order.LastCallbackError,
		CreatedAt:             order.CreatedAt,
		UpdatedAt:             order.UpdatedAt,
		PaidAt:                order.PaidAt,
		CallbackSuccessAt:     order.CallbackSuccessAt,
		LastCallbackAttemptAt: order.LastCallbackAttemptAt,
	}
}

func (o postgresOrder) toOrder() *Order {
	return &Order{
		OutTradeNo:            o.OutTradeNo,
		PID:                   o.PID,
		PaymentType:           o.PaymentType,
		Subject:               o.Subject,
		Amount:                o.Amount,
		NotifyURL:             o.NotifyURL,
		ReturnURL:             o.ReturnURL,
		AlipayTradeNo:         o.AlipayTradeNo,
		Status:                o.Status,
		CallbackAttempts:      o.CallbackAttempts,
		LastCallbackError:     o.LastCallbackError,
		CreatedAt:             o.CreatedAt,
		UpdatedAt:             o.UpdatedAt,
		PaidAt:                o.PaidAt,
		CallbackSuccessAt:     o.CallbackSuccessAt,
		LastCallbackAttemptAt: o.LastCallbackAttemptAt,
	}
}

func (s *PostgresOrderStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

var _ OrderStore = (*PostgresOrderStore)(nil)
var _ interface{ Close() error } = (*PostgresOrderStore)(nil)
