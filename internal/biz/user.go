package biz

import (
	"context"
	"strings"

	"gin-kratos-gorm/internal/biz/model"
	"gin-kratos-gorm/internal/pkg/apperr"
	"gin-kratos-gorm/internal/pkg/auth"

	"github.com/go-kratos/kratos/v2/log"
)

// UserRepository is implemented by the data layer.
type UserRepository interface {
	Create(ctx context.Context, user *model.User, roleIDs []int32) error
	FindByID(ctx context.Context, id int32) (*model.User, error)
	FindByName(ctx context.Context, name string) (*model.User, error)
	UpdateLastLogin(ctx context.Context, id int32) error
	UpdateRoles(ctx context.Context, userID int32, roleIDs []int32) error
}

type UserUsecase struct {
	repo UserRepository
	log  *log.Helper
}

func NewUserUsecase(repo UserRepository, logger log.Logger) *UserUsecase {
	return &UserUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (u *UserUsecase) Register(ctx context.Context, name, password, nickname string) (*model.User, error) {
	name = strings.TrimSpace(name)
	if len(name) < 3 || len(name) > 64 {
		return nil, apperr.Invalid("username must be between 3 and 64 characters")
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		Name:         name,
		Nickname:     nickname,
		PasswordHash: hash,
		Status:       1,
	}
	if err := u.repo.Create(ctx, user, []int32{DefaultRoleUserID}); err != nil {
		return nil, err
	}
	return u.repo.FindByID(ctx, user.ID)
}

func (u *UserUsecase) Login(ctx context.Context, name, password string) (*model.User, error) {
	user, err := u.repo.FindByName(ctx, strings.TrimSpace(name))
	if err != nil {
		// Do not leak whether the username exists.
		return nil, apperr.Unauthenticated("invalid username or password")
	}
	if user.Status != 1 {
		return nil, apperr.Forbidden("account is disabled")
	}
	if !auth.CheckPassword(user.PasswordHash, password) {
		return nil, apperr.Unauthenticated("invalid username or password")
	}
	if err := u.repo.UpdateLastLogin(ctx, user.ID); err != nil {
		u.log.Warnf("update last login for user %d: %v", user.ID, err)
	}
	return user, nil
}

func (u *UserUsecase) Get(ctx context.Context, id int32) (*model.User, error) {
	if id <= 0 {
		return nil, apperr.Invalid("user id must be positive")
	}
	return u.repo.FindByID(ctx, id)
}

func (u *UserUsecase) UpdateRoles(ctx context.Context, userID int32, roleIDs []int32) (*model.User, error) {
	if userID <= 0 {
		return nil, apperr.Invalid("user id must be positive")
	}
	if len(roleIDs) == 0 {
		return nil, apperr.Invalid("at least one role is required")
	}
	if err := u.repo.UpdateRoles(ctx, userID, roleIDs); err != nil {
		return nil, err
	}
	return u.repo.FindByID(ctx, userID)
}

func RoleIDs(user *model.User) []uint64 {
	ids := make([]uint64, 0, len(user.UserRoles))
	for _, ur := range user.UserRoles {
		if ur.Status != "" && ur.Status != "1" {
			continue
		}
		ids = append(ids, uint64(ur.RoleID))
	}
	return ids
}

func RoleCodes(user *model.User) []string {
	codes := make([]string, 0, len(user.UserRoles))
	for _, ur := range user.UserRoles {
		if ur.Status != "" && ur.Status != "1" {
			continue
		}
		if ur.Role.Code != "" {
			codes = append(codes, ur.Role.Code)
		}
	}
	return codes
}

const (
	DefaultRoleAdminID int32 = 1
	DefaultRoleUserID  int32 = 2
)
