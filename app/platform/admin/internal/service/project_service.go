package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"go.einride.tech/aip/filtering"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/pkg/aip/listing"
)

// ProjectServiceService 项目服务结构体
type ProjectServiceService struct {
	pb.UnimplementedProjectServiceServer
	puc *biz.ProjectUsecase
	log *log.Helper
}

// NewProjectServiceService creates a project service.
func NewProjectServiceService(puc *biz.ProjectUsecase, logger log.Logger) *ProjectServiceService {
	return &ProjectServiceService{
		puc: puc,
		log: log.NewHelper(logger),
	}
}

// ListProjects handles project list requests.
func (s *ProjectServiceService) ListProjects(ctx context.Context, req *pbCore.ListProjectsRequest) (*pbCore.ListProjectsResponse, error) {
	s.log.Infof("查询项目列表分页，page_size=%d page_token=%s", req.GetPageSize(), req.GetPageToken())
	params, err := listing.ParseParams(
		req,
		filtering.DeclareIdent("name", filtering.TypeString),
		filtering.DeclareIdent("code", filtering.TypeString),
		filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
	)
	if err != nil {
		return nil, err
	}
	req.PageSize = int32(params.PageSize)
	count, err := s.puc.CountProjects(ctx, listing.FilterOption(params.Filter))
	if err != nil {
		return nil, err
	}
	resp := pbCore.ListProjectsResponse{
		Total: count,
	}
	resp.Items, err = s.puc.ListProjects(ctx,
		listing.FilterOption(params.Filter),
		listing.OrderByOption(params.OrderBy),
		listing.LimitOption(params.PageSize),
		listing.OffsetOption(int(params.PageToken.Offset)),
	)
	if err != nil {
		return nil, err
	}
	if len(resp.Items) >= params.PageSize {
		resp.NextPageToken = params.PageToken.Next(req).String()
	}
	return &resp, nil
}

// GetProject handles get project requests.
func (s *ProjectServiceService) GetProject(ctx context.Context, req *pbCore.GetProjectRequest) (*pbCore.Project, error) {
	s.log.Infof("获取项目详情，项目ID：%v", req.GetId())
	return s.puc.Get(ctx, req.GetId())
}

// CreateProject handles create project requests.
func (s *ProjectServiceService) CreateProject(ctx context.Context, req *pbCore.CreateProjectRequest) (*pbCore.CreateProjectResponse, error) {
	s.log.Infof("创建项目，项目名称：%s", req.GetProject().GetName())
	_, err := s.puc.Create(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	return &pbCore.CreateProjectResponse{}, nil
}

// UpdateProject handles update project requests.
func (s *ProjectServiceService) UpdateProject(ctx context.Context, req *pbCore.UpdateProjectRequest) (*pbCore.UpdateProjectResponse, error) {
	s.log.Infof("更新项目，项目ID：%v", req.GetId())
	req.Project.Id = req.GetId()
	_, err := s.puc.Update(ctx, req.GetProject())
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateProjectResponse{}, nil
}

// DeleteProject handles delete project requests.
func (s *ProjectServiceService) DeleteProject(ctx context.Context, req *pbCore.DeleteProjectRequest) (*pbCore.DeleteProjectResponse, error) {
	s.log.Infof("删除项目，项目ID：%v", req.GetId())
	err := s.puc.Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pbCore.DeleteProjectResponse{}, nil
}

// UpdateProjectByStatus handles project status update requests.
func (s *ProjectServiceService) UpdateProjectByStatus(ctx context.Context, req *pbCore.UpdateProjectByStatusRequest) (*pbCore.UpdateProjectByStatusResponse, error) {
	s.log.Infof("更新项目状态，项目ID：%v，状态：%v", req.GetId(), req.GetStatus())
	err := s.puc.UpdateStatus(ctx, req.GetId(), req.GetStatus())
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateProjectByStatusResponse{}, nil
}
