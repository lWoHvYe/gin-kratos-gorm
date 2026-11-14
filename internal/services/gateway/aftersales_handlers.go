package gateway

import (
	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"
	"gin-kratos-gorm/internal/platform/id"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/metadata"
)

// createAfterSaleRequest is the body of POST /after-sales. user_id is always
// taken from the token.
type createAfterSaleRequest struct {
	OrderNo    string          `json:"order_no"`
	Type       string          `json:"type"`
	Reason     string          `json:"reason"`
	AmountCent int64           `json:"amount_cent"`
	Items      []afterSaleItem `json:"items"`
}

type afterSaleItem struct {
	SkuID    int64 `json:"sku_id"`
	Quantity int32 `json:"quantity"`
}

func (h *handlers) createAfterSale(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var in createAfterSaleRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	items := make([]*pb.AfterSaleItem, 0, len(in.Items))
	for _, item := range in.Items {
		items = append(items, &pb.AfterSaleItem{SkuId: item.SkuID, Quantity: item.Quantity})
	}
	orderNo := strings.TrimSpace(in.OrderNo)
	if orderNo == "" {
		httpx.Error(c, apperr.Invalid("order_no is required"))
		return
	}
	// The after-sales service does not consult orders, so ownership of the
	// order is checked here before a request can be opened against it.
	if _, err := h.clients.Order.GetOrder(c.Request.Context(), &pb.GetOrderRequest{OrderNo: orderNo, UserId: uid}); err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	// Forward an idempotency key so a double submit (or a retry after a network
	// error) returns the first after-sale instead of creating a second one.
	ctx := c.Request.Context()
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		key = id.Event()
	}
	if len(key) > 64 {
		key = key[:64]
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "x-client-request-id", key)
	afterSale, err := h.clients.AfterSales.CreateAfterSale(ctx, &pb.CreateAfterSaleRequest{
		OrderNo:    orderNo,
		UserId:     uid,
		Type:       strings.TrimSpace(in.Type),
		Reason:     in.Reason,
		AmountCent: in.AmountCent,
		Items:      items,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.Created(c, afterSale)
}

func (h *handlers) getAfterSale(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	afterSaleNo := strings.TrimSpace(c.Param("no"))
	if afterSaleNo == "" {
		httpx.Error(c, apperr.Invalid("after sale number is required"))
		return
	}
	afterSale, err := h.clients.AfterSales.GetAfterSale(c.Request.Context(), &pb.GetAfterSaleRequest{AfterSaleNo: afterSaleNo})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	if afterSale.GetUserId() != uid {
		httpx.Error(c, apperr.Forbidden("after sale belongs to another user"))
		return
	}
	httpx.OK(c, afterSale)
}

// ---------------------------------------------------------------- helpers

// userID returns the authenticated user. The auth middleware stores the claims
// in the request context; a missing or zero id is rejected.

type returnTrackingRequest struct {
	Carrier    string `json:"carrier"`
	TrackingNo string `json:"tracking_no"`
}

func (h *handlers) submitReturnTracking(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	no := strings.TrimSpace(c.Param("no"))
	var in returnTrackingRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	afterSale, err := h.clients.AfterSales.GetAfterSale(c.Request.Context(), &pb.GetAfterSaleRequest{AfterSaleNo: no})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	if afterSale.GetUserId() != uid {
		httpx.Error(c, apperr.Forbidden("after sale does not belong to user"))
		return
	}
	updated, err := h.clients.AfterSales.SubmitReturnTracking(c.Request.Context(), &pb.SubmitReturnTrackingRequest{
		AfterSaleNo: no, Carrier: strings.TrimSpace(in.Carrier), TrackingNo: strings.TrimSpace(in.TrackingNo),
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, updated)
}

func (h *handlers) listAfterSales(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	page, err := bff.QueryInt32(c, "page")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	pageSize, err := bff.QueryInt32(c, "page_size")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.AfterSales.ListAfterSales(c.Request.Context(), &pb.ListAfterSalesRequest{
		UserId: uid, Page: page, PageSize: pageSize, Status: strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}
