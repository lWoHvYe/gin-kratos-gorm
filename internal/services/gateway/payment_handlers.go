package gateway

import (
	"fmt"
	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"
	"gin-kratos-gorm/internal/platform/id"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// payRequest is the body of POST /payments/:paymentNo/pay. It simulates a
// channel callback; the status is always SUCCEEDED.
type payRequest struct {
	Channel    string `json:"channel"`
	EventID    string `json:"event_id"`
	AmountCent int64  `json:"amount_cent"`
	Signature  string `json:"signature"`
}

func (h *handlers) pay(c *gin.Context) {
	paymentNo := strings.TrimSpace(c.Param("paymentNo"))
	if paymentNo == "" {
		httpx.Error(c, apperr.Invalid("paymentNo is required"))
		return
	}
	var in payRequest
	if bff.BodyPresent(c) {
		if err := bff.BindJSON(c, &in); err != nil {
			httpx.Error(c, err)
			return
		}
	}
	eventID := strings.TrimSpace(in.EventID)
	if eventID == "" {
		eventID = strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	}
	if eventID == "" {
		eventID = id.Event()
	}
	channel := strings.TrimSpace(in.Channel)
	if channel == "" {
		channel = defaultPaymentChannel
	}
	// The simulated pay endpoint only receives payment_no. Load the intent so
	// the callback carries the authoritative amount and ownership can be
	// checked against the order.
	intent, err := h.clients.Payment.GetIntent(c.Request.Context(), &pb.GetIntentRequest{PaymentNo: paymentNo})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	if intent.GetOrderNo() != "" {
		order, oerr := h.clients.Order.GetOrder(c.Request.Context(), &pb.GetOrderRequest{OrderNo: intent.GetOrderNo(), UserId: uid})
		if oerr != nil {
			httpx.Error(c, bff.RemoteError(oerr))
			return
		}
		if order.GetUserId() != uid {
			httpx.Error(c, apperr.Forbidden("payment belongs to another user"))
			return
		}
		if order.GetStatus() != "PENDING_PAYMENT" {
			httpx.Error(c, apperr.Failed(fmt.Sprintf("order is not payable in status %s", order.GetStatus())))
			return
		}
	}
	if intent.GetExpiresAtUnix() > 0 && intent.GetExpiresAtUnix() < time.Now().Unix() {
		httpx.Error(c, apperr.Failed("payment intent has expired"))
		return
	}
	// The intent is authoritative. A client-supplied amount is only accepted
	// when it matches, so a caller cannot pick how much is settled.
	amount := intent.GetAmountCent()
	if in.AmountCent > 0 && in.AmountCent != amount {
		httpx.Error(c, apperr.Invalid("amount_cent does not match the payment intent"))
		return
	}
	reply, err := h.clients.Payment.Callback(c.Request.Context(), &pb.CallbackRequest{
		PaymentNo:  paymentNo,
		Channel:    channel,
		EventId:    eventID,
		AmountCent: amount,
		Status:     paymentSucceeded,
		Signature:  in.Signature,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

// ---------------------------------------------------------------- after sales

// createAfterSaleRequest is the body of POST /after-sales. user_id is always
// taken from the token.
