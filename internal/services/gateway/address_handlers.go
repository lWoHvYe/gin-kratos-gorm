package gateway

import (
	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"

	"github.com/gin-gonic/gin"
)

func (h *handlers) listAddresses(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Identity.ListAddresses(c.Request.Context(), &pb.GetUserRequest{UserId: uid})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) createAddress(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var address pb.Address
	if err := bff.BindJSON(c, &address); err != nil {
		httpx.Error(c, err)
		return
	}
	address.UserId = uid
	created, err := h.clients.Identity.CreateAddress(c.Request.Context(), &pb.CreateAddressRequest{Address: &address})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.Created(c, created)
}

func (h *handlers) updateAddress(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	addressID, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var address pb.Address
	if err := bff.BindJSON(c, &address); err != nil {
		httpx.Error(c, err)
		return
	}
	address.Id = addressID
	address.UserId = uid
	updated, err := h.clients.Identity.UpdateAddress(c.Request.Context(), &pb.UpdateAddressRequest{Address: &address})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, updated)
}

func (h *handlers) deleteAddress(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	addressID, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	if _, err := h.clients.Identity.DeleteAddress(c.Request.Context(), &pb.DeleteAddressRequest{UserId: uid, AddressId: addressID}); err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, gin.H{"status": "deleted"})
}

func (h *handlers) setDefaultAddress(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	addressID, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	if _, err := h.clients.Identity.SetDefaultAddress(c.Request.Context(), &pb.SetDefaultAddressRequest{UserId: uid, AddressId: addressID}); err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, gin.H{"status": "default"})
}

// ---------------------------------------------------------------- cart
