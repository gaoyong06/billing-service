package biz

import "time"

// DeductEvent is the message sent to RocketMQ for asynchronous batch processing
type DeductEvent struct {
	RecordID        string    `json:"record_id"`
	UserID          string    `json:"user_id"`
	ServiceName     string    `json:"service_name"`
	Count           int       `json:"count"`
	Cost            float64   `json:"cost"`
	FreeCount       int       `json:"free_count"`
	PaidCount       int       `json:"paid_count"`
	BalanceDeducted float64   `json:"balance_deducted"`
	DeductTime      time.Time `json:"deduct_time"`
	Month           string    `json:"month"` // Used to identify which month's quota/record this belongs to
}
