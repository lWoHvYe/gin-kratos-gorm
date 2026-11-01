package persistence

import (
	"context"
	"errors"
	"time"

	"gin-kratos-gorm/internal/biz/model"
	"gin-kratos-gorm/internal/pkg/apperr"

	"gorm.io/gorm"
)

type UserRepo struct {
	data *Data
}

func NewUserRepo(data *Data) *UserRepo {
	return &UserRepo{data: data}
}

func (r *UserRepo) Create(ctx context.Context, user *model.User, roleIDs []int32) error {
	if user == nil {
		return apperr.Invalid("user is required")
	}
	return r.data.InTx(ctx, func(ctx context.Context) error {
		if err := r.data.DB(ctx).Create(user).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return apperr.Conflict("username already exists")
			}
			return apperr.Internal("create user", err)
		}
		if len(roleIDs) == 0 {
			return nil
		}
		if err := r.validateRoles(ctx, roleIDs); err != nil {
			return err
		}
		links := make([]model.UserRole, 0, len(roleIDs))
		for _, roleID := range roleIDs {
			links = append(links, model.UserRole{UserID: user.ID, RoleID: roleID, Status: "1"})
		}
		if err := r.data.DB(ctx).Create(&links).Error; err != nil {
			return apperr.Internal("assign roles", err)
		}
		return nil
	})
}

func (r *UserRepo) FindByID(ctx context.Context, id int32) (*model.User, error) {
	var user model.User
	err := r.data.DB(ctx).
		Preload("UserRoles", "status = ?", "1").
		Preload("UserRoles.Role").
		Where("id = ?", id).
		First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("user not found")
	}
	if err != nil {
		return nil, apperr.Internal("find user", err)
	}
	return &user, nil
}

func (r *UserRepo) FindByName(ctx context.Context, name string) (*model.User, error) {
	var user model.User
	err := r.data.DB(ctx).
		Preload("UserRoles", "status = ?", "1").
		Preload("UserRoles.Role").
		Where("name = ?", name).
		First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("user not found")
	}
	if err != nil {
		return nil, apperr.Internal("find user", err)
	}
	return &user, nil
}

func (r *UserRepo) UpdateLastLogin(ctx context.Context, id int32) error {
	now := time.Now()
	err := r.data.DB(ctx).Model(&model.User{}).Where("id = ?", id).UpdateColumn("last_login_at", now).Error
	if err != nil {
		return apperr.Internal("update last login", err)
	}
	return nil
}

func (r *UserRepo) UpdateRoles(ctx context.Context, userID int32, roleIDs []int32) error {
	if len(roleIDs) == 0 {
		return apperr.Invalid("at least one role is required")
	}
	return r.data.InTx(ctx, func(ctx context.Context) error {
		var count int64
		if err := r.data.DB(ctx).Model(&model.User{}).Where("id = ?", userID).Count(&count).Error; err != nil {
			return apperr.Internal("count user", err)
		}
		if count == 0 {
			return apperr.NotFound("user not found")
		}
		if err := r.validateRoles(ctx, roleIDs); err != nil {
			return err
		}
		if err := r.data.DB(ctx).Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
			return apperr.Internal("delete user roles", err)
		}
		links := make([]model.UserRole, 0, len(roleIDs))
		seen := make(map[int32]struct{}, len(roleIDs))
		for _, roleID := range roleIDs {
			if _, ok := seen[roleID]; ok {
				continue
			}
			seen[roleID] = struct{}{}
			links = append(links, model.UserRole{UserID: userID, RoleID: roleID, Status: "1"})
		}
		if err := r.data.DB(ctx).Create(&links).Error; err != nil {
			return apperr.Internal("assign user roles", err)
		}
		return nil
	})
}

func (r *UserRepo) validateRoles(ctx context.Context, roleIDs []int32) error {
	seen := make(map[int32]struct{}, len(roleIDs))
	for _, roleID := range roleIDs {
		if _, ok := seen[roleID]; ok {
			continue
		}
		seen[roleID] = struct{}{}
		var role model.Role
		err := r.data.DB(ctx).Where("id = ?", roleID).First(&role).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.Invalid("role %d does not exist", roleID)
		}
		if err != nil {
			return apperr.Internal("find role", err)
		}
	}
	return nil
}
