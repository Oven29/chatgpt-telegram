package interfaces

import (
	"context"

	"github.com/m1guelpf/chatgpt-telegram/src/entities"
)

type PaymentProvider interface {
	CreatePayment(ctx context.Context, req entities.PaymentRequest) (*entities.PaymentResponse, error)
	VerifyPayment(ctx context.Context, req entities.PaymentVerificationRequest) (*entities.PaymentVerificationResponse, error)
}
