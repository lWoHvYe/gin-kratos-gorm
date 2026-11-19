package identity

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func normalizeUserPage(page, size *int32) {
	if *page <= 0 {
		*page = 1
	}
	if *size <= 0 {
		*size = 20
	}
	if *size > 100 {
		*size = 100
	}
}

func (s *Service) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.UserList, error) {
	var page, size int32 = req.GetPage(), req.GetPageSize()
	normalizeUserPage(&page, &size)
	where := "WHERE 1=1"
	args := []any{}
	if kw := strings.TrimSpace(req.GetKeyword()); kw != "" {
		where += " AND (username LIKE ? OR nickname LIKE ? OR phone LIKE ?)"
		like := "%" + kw + "%"
		args = append(args, like, like, like)
	}
	if status := strings.ToUpper(strings.TrimSpace(req.GetStatus())); status != "" {
		if status != "ACTIVE" && status != "DISABLED" {
			return nil, apperr.Invalid("status must be ACTIVE or DISABLED")
		}
		where += " AND status = ?"
		args = append(args, status)
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users "+where, args...).Scan(&total); err != nil {
		return nil, apperr.Internal("count users", err)
	}
	args = append(args, size, (page-1)*size)
	rows, err := s.db.QueryContext(ctx, `SELECT id, username, nickname, phone, status, created_at, token_version
		FROM users `+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, apperr.Internal("list users", err)
	}
	defer rows.Close()
	users := make([]*pb.User, 0, size)
	var ids []int64
	for rows.Next() {
		var user pb.User
		var created sql.NullTime
		if err := rows.Scan(&user.Id, &user.Username, &user.Nickname, &user.Phone, &user.Status, &created, &user.TokenVersion); err != nil {
			return nil, apperr.Internal("scan user", err)
		}
		if created.Valid {
			user.CreatedAtUnix = created.Time.Unix()
		}
		users = append(users, &user)
		ids = append(ids, user.Id)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate users", err)
	}
	if len(ids) > 0 {
		ph := userPlaceholders(len(ids))
		roleArgs := make([]any, len(ids))
		for i, id := range ids {
			roleArgs[i] = id
		}
		roleRows, err := s.db.QueryContext(ctx, `SELECT ur.user_id, r.code FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id WHERE ur.user_id IN (`+ph+`)`, roleArgs...)
		if err != nil {
			return nil, apperr.Internal("list user roles", err)
		}
		byID := make(map[int64]*pb.User, len(users))
		for _, user := range users {
			byID[user.Id] = user
		}
		for roleRows.Next() {
			var userID int64
			var code string
			if err := roleRows.Scan(&userID, &code); err != nil {
				roleRows.Close()
				return nil, apperr.Internal("scan user role", err)
			}
			if user := byID[userID]; user != nil {
				user.Roles = append(user.Roles, code)
			}
		}
		roleRows.Close()
		if err := roleRows.Err(); err != nil {
			return nil, apperr.Internal("iterate user roles", err)
		}
	}
	return &pb.UserList{Users: users, Total: total}, nil
}

func (s *Service) SetUserStatus(ctx context.Context, req *pb.SetUserStatusRequest) (*pb.User, error) {
	if req.GetUserId() <= 0 {
		return nil, apperr.Invalid("user_id is required")
	}
	status := strings.ToUpper(strings.TrimSpace(req.GetStatus()))
	if status != "ACTIVE" && status != "DISABLED" {
		return nil, apperr.Invalid("status must be ACTIVE or DISABLED")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin user status", err)
	}
	defer func() { _ = tx.Rollback() }()
	var current string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM users WHERE id = ? FOR UPDATE`, req.GetUserId()).Scan(&current); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("user not found")
		}
		return nil, apperr.Internal("lock user", err)
	}
	if status == "DISABLED" {
		var isAdmin int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id WHERE ur.user_id = ? AND r.code = 'admin'`, req.GetUserId()).Scan(&isAdmin); err != nil {
			return nil, apperr.Internal("check admin role", err)
		}
		if isAdmin > 0 {
			var others int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users u
				JOIN user_roles ur ON ur.user_id = u.id JOIN roles r ON r.id = ur.role_id
				WHERE r.code = 'admin' AND u.status = 'ACTIVE' AND u.id <> ?`, req.GetUserId()).Scan(&others); err != nil {
				return nil, apperr.Internal("count active admins", err)
			}
			if others == 0 {
				return nil, apperr.Conflict("cannot disable the last active admin")
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET status = ?, token_version = token_version + 1, updated_at = NOW(3) WHERE id = ?`, status, req.GetUserId()); err != nil {
		return nil, apperr.Internal("update user status", err)
	}
	if status == "DISABLED" {
		if _, err := tx.ExecContext(ctx, `UPDATE refresh_tokens SET revoked_at = NOW(3)
			WHERE user_id = ? AND revoked_at IS NULL`, req.GetUserId()); err != nil {
			return nil, apperr.Internal("revoke refresh tokens", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit user status", err)
	}
	return s.GetUser(ctx, &pb.GetUserRequest{UserId: req.GetUserId()})
}

func userPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}
