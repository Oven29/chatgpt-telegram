package interfaces

import (
	"context"
)

type PaymentProvider interface {
	CreatePayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error)
	VerifyPayment(ctx context.Context, req PaymentVerificationRequest) (*PaymentVerificationResponse, error)
}
