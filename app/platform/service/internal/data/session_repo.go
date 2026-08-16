package data

import (
	"context"
	stderrors "errors"
	"sort"
	"strings"

	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/auth/session"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

type sessionRepo struct {
	token *session.Manager
	log   *log.Helper
}

func NewSessionRepo(token *session.Manager, logger log.Logger) biz.SessionRepo {
	return &sessionRepo{token: token, log: log.NewHelper(logger)}
}

func sessionProto(item *session.Session, currentSessionID string) *pb.Session {
	createdAt := item.CreatedAt.Format(timeFormat)
	lastActiveAt := item.LastActiveAt.Format(timeFormat)
	expiresAt := item.ExpiresAt.Format(timeFormat)
	return &pb.Session{
		Id:           item.ID,
		TenantId:     item.TenantID,
		UserId:       item.UserID,
		Username:     item.Username,
		Ip:           &item.IP,
		UserAgent:    &item.UserAgent,
		Current:      item.ID == currentSessionID,
		CreatedAt:    &createdAt,
		LastActiveAt: &lastActiveAt,
		ExpiresAt:    &expiresAt,
	}
}

const timeFormat = "2006-01-02 15:04:05"

func (r *sessionRepo) List(ctx context.Context, req *pb.ListSessionsRequest, currentSessionID string) ([]*pb.Session, int32, error) {
	tenantID, err := requireTenantID(ctx)
	if err != nil {
		return nil, 0, err
	}
	sessions, err := r.token.ListTenantSessions(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}
	filtered := make([]*session.Session, 0, len(sessions))
	for _, item := range sessions {
		if req.UserId != nil && item.UserID != *req.UserId {
			continue
		}
		if req.Username != nil && !strings.Contains(strings.ToLower(item.Username), strings.ToLower(*req.Username)) {
			continue
		}
		if req.Ip != nil && item.IP != *req.Ip {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].LastActiveAt.After(filtered[j].LastActiveAt)
	})
	total := len(filtered)
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	if offset > total {
		offset = total
	}
	end := offset + size
	if end > total {
		end = total
	}
	result := make([]*pb.Session, 0, end-offset)
	for _, item := range filtered[offset:end] {
		result = append(result, sessionProto(item, currentSessionID))
	}
	return result, int32(total), nil
}

func (r *sessionRepo) ListMine(ctx context.Context, userID uint32, currentSessionID string) ([]*pb.Session, error) {
	tenantID, err := requireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	sessions, err := r.token.ListUserSessions(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastActiveAt.After(sessions[j].LastActiveAt)
	})
	result := make([]*pb.Session, 0, len(sessions))
	for _, item := range sessions {
		result = append(result, sessionProto(item, currentSessionID))
	}
	return result, nil
}

func (r *sessionRepo) Revoke(ctx context.Context, sessionID string) error {
	tenantID, err := requireTenantID(ctx)
	if err != nil {
		return err
	}
	if err := r.token.RevokeSession(ctx, tenantID, sessionID); err != nil {
		if stderrors.Is(err, session.ErrSessionNotFound) {
			return errors.NotFound("SESSION_NOT_FOUND", "会话不存在")
		}
		return err
	}
	return nil
}

func (r *sessionRepo) RevokeUser(ctx context.Context, userID uint32) error {
	tenantID, err := requireTenantID(ctx)
	if err != nil {
		return err
	}
	return r.token.RevokeUserSessions(ctx, tenantID, userID)
}

func (r *sessionRepo) RevokeTenant(ctx context.Context, tenantID uint32) error {
	return r.token.RevokeTenantSessions(ctx, tenantID)
}
