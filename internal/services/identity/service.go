package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/db"

	"github.com/go-sql-driver/mysql"
)

type Service struct {
	pb.UnimplementedIdentityServiceServer
	db  *sql.DB
	jwt *auth.Manager
	log *slog.Logger
}

func New(db *sql.DB, jwt *auth.Manager, logger *slog.Logger) *Service {
	return &Service{db: db, jwt: jwt, log: logger}
}

func (s *Service) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.LoginReply, error) {
	username := strings.TrimSpace(req.GetUsername())
	if len(username) < 3 || len(username) > 64 {
		return nil, apperr.Invalid("username must be 3-64 characters")
	}
	hash, err := auth.HashPassword(req.GetPassword())
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin register", err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `INSERT INTO users (username, password_hash, nickname) VALUES (?, ?, ?)`, username, hash, req.GetNickname())
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			return nil, apperr.Conflict("username already exists")
		}
		return nil, apperr.Internal("create user", err)
	}
	id, _ := res.LastInsertId()
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES (?, 2)`, id); err != nil {
		return nil, apperr.Internal("assign role", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit register", err)
	}
	return s.loginReply(ctx, id, username, req.GetNickname(), []string{"user"})
}

func (s *Service) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginReply, error) {
	var (
		id                               int64
		username, nickname, hash, status string
	)
	err := s.db.QueryRowContext(ctx, `SELECT id, username, nickname, password_hash, status FROM users WHERE username = ?`, strings.TrimSpace(req.GetUsername())).
		Scan(&id, &username, &nickname, &hash, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.Unauthenticated("invalid username or password")
	}
	if err != nil {
		return nil, apperr.Internal("query user", err)
	}
	if status != "ACTIVE" {
		return nil, apperr.Forbidden("account disabled")
	}
	if !auth.CheckPassword(hash, req.GetPassword()) {
		return nil, apperr.Unauthenticated("invalid username or password")
	}
	roles, err := s.roles(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.loginReply(ctx, id, username, nickname, roles)
}

func (s *Service) Refresh(ctx context.Context, req *pb.RefreshRequest) (*pb.LoginReply, error) {
	raw := strings.TrimSpace(req.GetRefreshToken())
	if raw == "" {
		return nil, apperr.Unauthenticated("refresh token is required")
	}
	hash := auth.HashRefresh(raw)
	var userID int64
	err := s.db.QueryRowContext(ctx, `SELECT user_id FROM refresh_tokens
		WHERE token_hash = ? AND revoked_at IS NULL AND expires_at > NOW(3)`, hash).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.Unauthenticated("invalid or expired refresh token")
	}
	if err != nil {
		return nil, apperr.Internal("query refresh token", err)
	}
	res, err := s.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked_at = NOW(3)
		WHERE token_hash = ? AND revoked_at IS NULL`, hash)
	if err != nil {
		return nil, apperr.Internal("rotate refresh token", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, apperr.Unauthenticated("refresh token already used")
	}
	var username, nickname string
	if err := s.db.QueryRowContext(ctx, `SELECT username, nickname FROM users WHERE id = ? AND status = 'ACTIVE'`, userID).Scan(&username, &nickname); err != nil {
		return nil, apperr.Unauthenticated("account is not active")
	}
	roles, err := s.roles(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.loginReply(ctx, userID, username, nickname, roles)
}

func (s *Service) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.Empty, error) {
	if raw := strings.TrimSpace(req.GetRefreshToken()); raw != "" {
		if _, err := s.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked_at = NOW(3)
			WHERE token_hash = ? AND revoked_at IS NULL`, auth.HashRefresh(raw)); err != nil {
			return nil, apperr.Internal("revoke refresh token", err)
		}
	}
	return &pb.Empty{}, nil
}

func (s *Service) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	var user pb.User
	err := s.db.QueryRowContext(ctx, `SELECT id, username, nickname, phone FROM users WHERE id = ?`, req.GetUserId()).
		Scan(&user.Id, &user.Username, &user.Nickname, &user.Phone)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("user not found")
	}
	if err != nil {
		return nil, apperr.Internal("query user", err)
	}
	roles, err := s.roles(ctx, user.Id)
	if err != nil {
		return nil, err
	}
	user.Roles = roles
	return &user, nil
}

func (s *Service) CreateAddress(ctx context.Context, req *pb.CreateAddressRequest) (*pb.Address, error) {
	a := req.GetAddress()
	if a == nil || a.GetUserId() <= 0 {
		return nil, apperr.Invalid("address and user_id are required")
	}
	if strings.TrimSpace(a.GetReceiver()) == "" || strings.TrimSpace(a.GetDetail()) == "" {
		return nil, apperr.Invalid("receiver and detail are required")
	}
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		if a.GetIsDefault() {
			if _, err := tx.ExecContext(ctx, `UPDATE user_addresses SET is_default = 0 WHERE user_id = ?`, a.GetUserId()); err != nil {
				return apperr.Internal("clear default address", err)
			}
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO user_addresses (user_id, receiver, phone, province, city, district, detail, is_default)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, a.GetUserId(), a.GetReceiver(), a.GetPhone(), a.GetProvince(), a.GetCity(), a.GetDistrict(), a.GetDetail(), a.GetIsDefault())
		if err != nil {
			return apperr.Internal("create address", err)
		}
		a.Id, _ = res.LastInsertId()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Service) UpdateAddress(ctx context.Context, req *pb.UpdateAddressRequest) (*pb.Address, error) {
	a := req.GetAddress()
	if a == nil || a.GetId() <= 0 || a.GetUserId() <= 0 {
		return nil, apperr.Invalid("address id and user_id are required")
	}
	if strings.TrimSpace(a.GetReceiver()) == "" || strings.TrimSpace(a.GetDetail()) == "" {
		return nil, apperr.Invalid("receiver and detail are required")
	}
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		if a.GetIsDefault() {
			if _, err := tx.ExecContext(ctx, `UPDATE user_addresses SET is_default = 0 WHERE user_id = ?`, a.GetUserId()); err != nil {
				return apperr.Internal("clear default address", err)
			}
		}
		res, err := tx.ExecContext(ctx, `UPDATE user_addresses
			SET receiver = ?, phone = ?, province = ?, city = ?, district = ?, detail = ?, is_default = ?
			WHERE id = ? AND user_id = ?`,
			a.GetReceiver(), a.GetPhone(), a.GetProvince(), a.GetCity(), a.GetDistrict(), a.GetDetail(), a.GetIsDefault(), a.GetId(), a.GetUserId())
		if err != nil {
			return apperr.Internal("update address", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return apperr.NotFound("address not found")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Service) DeleteAddress(ctx context.Context, req *pb.DeleteAddressRequest) (*pb.Empty, error) {
	if req.GetUserId() <= 0 || req.GetAddressId() <= 0 {
		return nil, apperr.Invalid("user_id and address_id are required")
	}
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM user_addresses WHERE id = ? AND user_id = ?`, req.GetAddressId(), req.GetUserId())
		if err != nil {
			return apperr.Internal("delete address", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return apperr.NotFound("address not found")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

func (s *Service) SetDefaultAddress(ctx context.Context, req *pb.SetDefaultAddressRequest) (*pb.Empty, error) {
	if req.GetUserId() <= 0 || req.GetAddressId() <= 0 {
		return nil, apperr.Invalid("user_id and address_id are required")
	}
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_addresses WHERE id = ? AND user_id = ?`, req.GetAddressId(), req.GetUserId()).Scan(&exists); err != nil {
			return apperr.Internal("query address", err)
		}
		if exists == 0 {
			return apperr.NotFound("address not found")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE user_addresses SET is_default = 0 WHERE user_id = ?`, req.GetUserId()); err != nil {
			return apperr.Internal("clear default address", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE user_addresses SET is_default = 1 WHERE id = ? AND user_id = ?`, req.GetAddressId(), req.GetUserId()); err != nil {
			return apperr.Internal("set default address", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

func (s *Service) ListAddresses(ctx context.Context, req *pb.GetUserRequest) (*pb.AddressList, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, receiver, phone, province, city, district, detail, is_default
		FROM user_addresses WHERE user_id = ? ORDER BY is_default DESC, id DESC`, req.GetUserId())
	if err != nil {
		return nil, apperr.Internal("list addresses", err)
	}
	defer rows.Close()
	reply := &pb.AddressList{}
	for rows.Next() {
		var a pb.Address
		if err := rows.Scan(&a.Id, &a.UserId, &a.Receiver, &a.Phone, &a.Province, &a.City, &a.District, &a.Detail, &a.IsDefault); err != nil {
			return nil, apperr.Internal("scan address", err)
		}
		reply.Addresses = append(reply.Addresses, &a)
	}
	return reply, rows.Err()
}

func (s *Service) roles(ctx context.Context, userID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT r.code FROM roles r JOIN user_roles ur ON ur.role_id = r.id WHERE ur.user_id = ?`, userID)
	if err != nil {
		return nil, apperr.Internal("query roles", err)
	}
	defer rows.Close()
	var roles []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, apperr.Internal("scan role", err)
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

func (s *Service) loginReply(ctx context.Context, id int64, username, nickname string, roles []string) (*pb.LoginReply, error) {
	access, refresh, err := s.jwt.Generate(uint64(id), roles)
	if err != nil {
		return nil, err
	}
	hash := auth.HashRefresh(refresh)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES (?, ?, ?)`, id, hash, time.Now().Add(s.jwt.RefreshTTL())); err != nil {
		return nil, apperr.Internal("store refresh token", err)
	}
	return &pb.LoginReply{
		AccessToken: access, RefreshToken: refresh,
		User: &pb.User{Id: id, Username: username, Nickname: nickname, Roles: roles},
	}, nil
}

var _ = fmt.Sprintf
