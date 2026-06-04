package service

import (
	"context"

	pb "backend-service/api/avmc/admin/v1"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"go.einride.tech/aip/filtering"
	"go.einride.tech/aip/ordering"
	"go.einride.tech/aip/pagination"
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
	pageSize := biz.NormalizePageSize(req.GetPageSize())
	req.PageSize = int32(pageSize)

	declarations, err := filtering.NewDeclarations(
		filtering.DeclareStandardFunctions(),
		filtering.DeclareIdent("name", filtering.TypeString),
		filtering.DeclareIdent("code", filtering.TypeString),
		filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
	)
	if err != nil {
		return nil, err
	}
	filter, err := filtering.ParseFilter(req, declarations)
	if err != nil {
		return nil, err
	}

	pageToken, err := pagination.ParsePageToken(req)
	if err != nil {
		return nil, err
	}
	orderBy, err := ordering.ParseOrderBy(req)
	if err != nil {
		return nil, err
	}
	count, err := s.puc.CountProjects(ctx, biz.ListFilter(filter))
	if err != nil {
		return nil, err
	}
	resp := pbCore.ListProjectsResponse{
		Total: count,
	}
	resp.Items, err = s.puc.ListProjects(ctx,
		biz.ListFilter(filter),
		biz.ListOrderBy(orderBy),
		biz.ListLimit(pageSize),
		biz.ListOffset(int(pageToken.Offset)),
	)
	if err != nil {
		return nil, err
	}
	if len(resp.Items) >= pageSize {
		resp.NextPageToken = pageToken.Next(req).String()
	}
	return &resp, nil
}

// GetProject handles get project requests.
func (s *ProjectServiceService) GetProject(ctx context.Context, req *pbCore.GetProjectRequest) (*pbCore.Project, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorProjectInvalidId("项目ID不能为空")
	}
	s.log.Infof("获取项目详情，项目ID：%v", req.GetId())
	return s.puc.Get(ctx, req.GetId())
}

// CreateProject handles create project requests.
func (s *ProjectServiceService) CreateProject(ctx context.Context, req *pbCore.CreateProjectRequest) (*pbCore.CreateProjectResponse, error) {
	if req.GetProject() == nil {
		return nil, pb.ErrorProjectInvalidId("项目信息不能为空")
	}
	if req.GetProject().GetName() == "" {
		return nil, pb.ErrorProjectNameCannotBeEmpty("项目名称不能为空")
	}
	s.log.Infof("创建项目，项目名称：%s", req.GetProject().GetName())
	_, err := s.puc.Create(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	return &pbCore.CreateProjectResponse{}, nil
}

// UpdateProject handles update project requests.
func (s *ProjectServiceService) UpdateProject(ctx context.Context, req *pbCore.UpdateProjectRequest) (*pbCore.UpdateProjectResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorProjectInvalidId("项目ID不能为空")
	}
	if req.GetProject() == nil {
		return nil, pb.ErrorProjectInvalidId("项目信息不能为空")
	}
	if req.GetProject().GetName() == "" {
		return nil, pb.ErrorProjectNameCannotBeEmpty("项目名称不能为空")
	}
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
	if req.GetId() == 0 {
		return nil, pb.ErrorProjectInvalidId("项目ID不能为空")
	}
	s.log.Infof("删除项目，项目ID：%v", req.GetId())
	err := s.puc.Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pbCore.DeleteProjectResponse{}, nil
}

// UpdateProjectByStatus handles project status update requests.
func (s *ProjectServiceService) UpdateProjectByStatus(ctx context.Context, req *pbCore.UpdateProjectByStatusRequest) (*pbCore.UpdateProjectByStatusResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorProjectInvalidId("项目ID不能为空")
	}
	if req.GetStatus() < 1 || req.GetStatus() > 2 {
		return nil, pb.ErrorProjectStatusInvalid("项目状态无效")
	}
	s.log.Infof("更新项目状态，项目ID：%v，状态：%v", req.GetId(), req.GetStatus())
	err := s.puc.UpdateStatus(ctx, req.GetId(), req.GetStatus())
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateProjectByStatusResponse{}, nil
}
