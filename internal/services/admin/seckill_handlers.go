package admin

import (
	"context"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// ---------------------------------------------------------------- seckill

func (h *handlers) listSeckillActivities(c *gin.Context) {
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
	reply, err := h.clients.Seckill.ListActivities(c.Request.Context(), &pb.ListSeckillActivitiesRequest{
		Page: page, PageSize: size, Status: strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) createSeckillActivity(c *gin.Context) {
	var in pb.CreateSeckillActivityRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Seckill.CreateActivity(c.Request.Context(), &in)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.Created(c, reply)
}

func (h *handlers) getSeckillActivity(c *gin.Context) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Seckill.GetActivity(c.Request.Context(), &pb.GetSeckillActivityRequest{Id: id})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) updateSeckillActivity(c *gin.Context) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var in pb.UpdateSeckillActivityRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	in.Id = id
	reply, err := h.clients.Seckill.UpdateActivity(c.Request.Context(), &in)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) publishSeckillActivity(c *gin.Context) {
	h.seckillCommand(c, h.clients.Seckill.PublishActivity)
}

func (h *handlers) pauseSeckillActivity(c *gin.Context) {
	h.seckillCommand(c, h.clients.Seckill.PauseActivity)
}

func (h *handlers) warmupSeckillActivity(c *gin.Context) {
	h.seckillCommand(c, h.clients.Seckill.WarmupActivity)
}

func (h *handlers) seckillCommand(c *gin.Context, fn func(context.Context, *pb.SeckillActivityCommand, ...grpc.CallOption) (*pb.SeckillActivity, error)) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := fn(c.Request.Context(), &pb.SeckillActivityCommand{Id: id})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) getSeckillStock(c *gin.Context) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Seckill.GetActivityStock(c.Request.Context(), &pb.SeckillActivityCommand{Id: id})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) replaySeckillDLQ(c *gin.Context) {
	var in pb.ReplaySeckillDLQRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Seckill.ReplayDLQ(c.Request.Context(), &in)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}
