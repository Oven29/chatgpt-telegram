package adapters

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/m1guelpf/chatgpt-telegram/src/entities"
)

type FreeKassaProvider struct {
	MerchantID   string
	SecretWord1 string // для проверки подписи
	SecretWord2 string // для генерации ссылки
	BaseURL      string
	APIKey       string // для запроса к API freekassa.ru
	APIBaseURL   string // по умолчанию https://api.freekassa.ru/v1
	HTTPClient   *http.Client
}

func NewFreeKassaProvider(
	merchantID string,
	secret1 string,
	secret2 string,
	apiKey string,
) *FreeKassaProvider {
	return &FreeKassaProvider{
		MerchantID:   merchantID,
		SecretWord1:  secret1,
		SecretWord2:  secret2,
		APIKey:       apiKey,
		BaseURL:      "https://pay.freekassa.ru/",
		APIBaseURL:   "https://api.freekassa.ru/v1",
		HTTPClient:   http.DefaultClient,
	}
}

func (fk *FreeKassaProvider) CreatePayment(ctx context.Context, req entities.PaymentRequest) (*entities.PaymentResponse, error) {
	amountStr := fmt.Sprintf("%.2f", req.Amount)
	signData := fmt.Sprintf("%s:%s:%s:%d", fk.MerchantID, amountStr, fk.SecretWord2, req.OrderID)
	sign := md5Hash(signData)

	url := fmt.Sprintf("%s?m=%s&o=%d&oa=%s&s=%s&email=%s",
		fk.BaseURL, fk.MerchantID, req.OrderID, amountStr, sign, req.Email,
	)

	return &entities.PaymentResponse{
		PaymentURL: url,
	}, nil
}

func (fk *FreeKassaProvider) VerifyPayment(ctx context.Context, req entities.PaymentVerificationRequest) (*entities.PaymentVerificationResponse, error) {
	if fk.APIKey == "" {
		return nil, fmt.Errorf("FreeKassa API key not set")
	}
	if fk.APIBaseURL == "" {
		fk.APIBaseURL = "https://api.freekassa.ru/v1"
	}
	if fk.HTTPClient == nil {
		fk.HTTPClient = http.DefaultClient
	}

	url := fmt.Sprintf("%s/orders?merchant_order_id=%s", fk.APIBaseURL, req.OrderID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+fk.APIKey)

	resp, err := fk.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var apiResp struct {
		Type   string `json:"type"`
		Orders []struct {
			ID              int    `json:"id"`
			MerchantOrderID string `json:"merchant_order_id"`
			Status          string `json:"status"` // "success", "fail", etc.
			Amount          string `json:"amount"`
		} `json:"orders"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Orders) == 0 {
		return nil, fmt.Errorf("no order found")
	}

	order := apiResp.Orders[0]
	isPaid := order.Status == "success"

	return &entities.PaymentVerificationResponse{
		IsPaid:   isPaid,
		OrderID: order.MerchantOrderID,
		Amount:  order.Amount,
	}, nil
}

func md5Hash(s string) string {
	hash := md5.Sum([]byte(s))
	return hex.EncodeToString(hash[:])
}
