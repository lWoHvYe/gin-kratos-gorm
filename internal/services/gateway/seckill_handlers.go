package gateway

import (
	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (h *handlers) getSeckillActivity(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(c, apperr.Invalid("invalid activity id"))
		return
	}
	reply, err := h.clients.Seckill.GetPublicActivity(c.Request.Context(), &pb.GetSeckillActivityRequest{Id: id})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

type reserveSeckillRequest struct {
	AddressID       int64  `json:"address_id"`
	ReceiverName    string `json:"receiver_name"`
	ReceiverPhone   string `json:"receiver_phone"`
	ReceiverAddress string `json:"receiver_address"`
}

func (h *handlers) reserveSeckill(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(c, apperr.Invalid("invalid activity id"))
		return
	}
	var in reserveSeckillRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Seckill.Reserve(c.Request.Context(), &pb.ReserveSeckillRequest{
		ActivityId: id, UserId: uid, AddressId: in.AddressID,
		ReceiverName: in.ReceiverName, ReceiverPhone: in.ReceiverPhone, ReceiverAddress: in.ReceiverAddress,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	c.JSON(http.StatusAccepted, reply)
}

func (h *handlers) getSeckillQualification(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Seckill.GetQualification(c.Request.Context(), &pb.GetSeckillQualificationRequest{
		Token: c.Param("token"), UserId: uid,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) listPublicSeckillActivities(c *gin.Context) {
	page, err := bff.QueryInt32(c, "page")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	size, err := bff.QueryInt32(c, "page_size")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Seckill.ListPublicActivities(c.Request.Context(), &pb.ListSeckillActivitiesRequest{
		Page: page, PageSize: size,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}
