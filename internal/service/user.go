package service

import (
	"context"
	"time"

	pb "gin-kratos-gorm/api/user/v1"
	"gin-kratos-gorm/internal/biz"
	"gin-kratos-gorm/internal/biz/model"
	"gin-kratos-gorm/internal/pkg/apperr"
	"gin-kratos-gorm/internal/pkg/auth"
)

type UserService struct {
	pb.UnimplementedUserServer
	uc  *biz.UserUsecase
	jwt *auth.Manager
}

func NewUserService(uc *biz.UserUsecase, jwtManager *auth.Manager) *UserService {
	return &UserService{uc: uc, jwt: jwtManager}
}

func (s *UserService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.UserReply, error) {
	user, err := s.uc.Register(ctx, req.GetName(), req.GetPassword(), req.GetNickname())
	if err != nil {
		return nil, err
	}
	return s.replyWithToken(user)
}

func (s *UserService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.UserReply, error) {
	user, err := s.uc.Login(ctx, req.GetName(), req.GetPassword())
	if err != nil {
		return nil, err
	}
	return s.replyWithToken(user)
}

func (s *UserService) GetByID(ctx context.Context, req *pb.SingleIDRequest) (*pb.UserReply, error) {
	user, err := s.uc.Get(ctx, int32(req.GetId()))
	if err != nil {
		return nil, err
	}
	return toUserReply(user, ""), nil
}

func (s *UserService) UpdateRoleByID(ctx context.Context, req *pb.UpdateRoleRequest) (*pb.UserReply, error) {
	if !auth.HasRole(ctx, "admin") {
		return nil, apperr.Forbidden("admin role required")
	}
	roleIDs := make([]int32, 0, len(req.GetRoleId()))
	for _, id := range req.GetRoleId() {
		roleIDs = append(roleIDs, int32(id))
	}
	user, err := s.uc.UpdateRoles(ctx, int32(req.GetId()), roleIDs)
	if err != nil {
		return nil, err
	}
	return toUserReply(user, ""), nil
}

func (s *UserService) replyWithToken(user *model.User) (*pb.UserReply, error) {
	token, _, err := s.jwt.Generate(uint64(user.ID), user.Name, biz.RoleIDs(user), biz.RoleCodes(user))
	if err != nil {
		return nil, err
	}
	reply := toUserReply(user, token)
	return reply, nil
}

func toUserReply(user *model.User, token string) *pb.UserReply {
	if user == nil {
		return &pb.UserReply{}
	}
	reply := &pb.UserReply{
		Id:        uint64(user.ID),
		Name:      user.Name,
		Nickname:  user.Nickname,
		RoleId:    biz.RoleIDs(user),
		RoleCodes: biz.RoleCodes(user),
		Token:     token,
	}
	if !user.CreatedAt.IsZero() {
		reply.CreatedAt = user.CreatedAt.Format(time.RFC3339)
	}
	return reply
}
