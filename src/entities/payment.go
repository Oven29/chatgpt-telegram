package entities

type PaymentRequest struct {
	OrderID int
	Amount  float64
	Email   string
}

type PaymentResponse struct {
	PaymentURL string
}

type PaymentVerificationRequest struct {
	OrderID string
	Amount  string
	Sign    string
}

type PaymentVerificationResponse struct {
	IsPaid   bool
	OrderID string
	Amount  string
}
