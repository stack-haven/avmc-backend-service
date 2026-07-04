package service

import (
	"context"
	"strconv"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
)

type ParameterServiceService struct {
	pb.UnimplementedParameterServiceServer
	uc *biz.ParameterUsecase
}

func NewParameterServiceService(uc *biz.ParameterUsecase) *ParameterServiceService {
	return &ParameterServiceService{uc: uc}
}

func (s *ParameterServiceService) ListParameterDefinitions(ctx context.Context, req *pbCore.ListParameterDefinitionsRequest) (*pbCore.ListParameterDefinitionsResponse, error) {
	items, total, err := s.uc.ListDefinitions(ctx, req)
	if err != nil {
		return nil, err
	}
	resp := &pbCore.ListParameterDefinitionsResponse{Items: items, Total: total}
	offset, _ := strconv.Atoi(req.GetPageToken())
	if offset+len(items) < int(total) {
		resp.NextPageToken = strconv.Itoa(offset + len(items))
	}
	return resp, nil
}

func (s *ParameterServiceService) GetParameterDefinition(ctx context.Context, req *pbCore.GetParameterDefinitionRequest) (*pbCore.GetParameterDefinitionResponse, error) {
	item, err := s.uc.GetDefinition(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pbCore.GetParameterDefinitionResponse{Parameter: item}, nil
}

func (s *ParameterServiceService) CreateParameterDefinition(ctx context.Context, req *pbCore.CreateParameterDefinitionRequest) (*pbCore.CreateParameterDefinitionResponse, error) {
	item, err := s.uc.CreateDefinition(ctx, req.GetParameter())
	if err != nil {
		return nil, err
	}
	return &pbCore.CreateParameterDefinitionResponse{Parameter: item}, nil
}

func (s *ParameterServiceService) UpdateParameterDefinition(ctx context.Context, req *pbCore.UpdateParameterDefinitionRequest) (*pbCore.UpdateParameterDefinitionResponse, error) {
	req.Parameter.Id = req.GetId()
	item, err := s.uc.UpdateDefinition(ctx, req.GetParameter())
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateParameterDefinitionResponse{Parameter: item}, nil
}

func (s *ParameterServiceService) DeleteParameterDefinition(ctx context.Context, req *pbCore.DeleteParameterDefinitionRequest) (*pbCore.DeleteParameterDefinitionResponse, error) {
	if err := s.uc.DeleteDefinition(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pbCore.DeleteParameterDefinitionResponse{}, nil
}

func (s *ParameterServiceService) ListCurrentTenantParameters(ctx context.Context, req *pbCore.ListCurrentTenantParametersRequest) (*pbCore.ListCurrentTenantParametersResponse, error) {
	items, err := s.uc.ListCurrent(ctx, req.GetKey())
	if err != nil {
		return nil, err
	}
	return &pbCore.ListCurrentTenantParametersResponse{Items: items}, nil
}

func (s *ParameterServiceService) SetCurrentTenantParameter(ctx context.Context, req *pbCore.SetCurrentTenantParameterRequest) (*pbCore.SetCurrentTenantParameterResponse, error) {
	item, err := s.uc.SetCurrent(ctx, req.GetKey(), req.GetValue())
	if err != nil {
		return nil, err
	}
	return &pbCore.SetCurrentTenantParameterResponse{Parameter: item}, nil
}

func (s *ParameterServiceService) ResetCurrentTenantParameter(ctx context.Context, req *pbCore.ResetCurrentTenantParameterRequest) (*pbCore.ResetCurrentTenantParameterResponse, error) {
	if err := s.uc.ResetCurrent(ctx, req.GetKey()); err != nil {
		return nil, err
	}
	return &pbCore.ResetCurrentTenantParameterResponse{}, nil
}

func (s *ParameterServiceService) ListTenantParameters(ctx context.Context, req *pbCore.ListTenantParametersRequest) (*pbCore.ListTenantParametersResponse, error) {
	items, err := s.uc.ListTenant(ctx, req.GetTenantId(), req.GetKey())
	if err != nil {
		return nil, err
	}
	return &pbCore.ListTenantParametersResponse{Items: items}, nil
}

func (s *ParameterServiceService) SetTenantParameter(ctx context.Context, req *pbCore.SetTenantParameterRequest) (*pbCore.SetTenantParameterResponse, error) {
	item, err := s.uc.SetTenant(ctx, req.GetTenantId(), req.GetKey(), req.GetValue())
	if err != nil {
		return nil, err
	}
	return &pbCore.SetTenantParameterResponse{Parameter: item}, nil
}

func (s *ParameterServiceService) ResetTenantParameter(ctx context.Context, req *pbCore.ResetTenantParameterRequest) (*pbCore.ResetTenantParameterResponse, error) {
	if err := s.uc.ResetTenant(ctx, req.GetTenantId(), req.GetKey()); err != nil {
		return nil, err
	}
	return &pbCore.ResetTenantParameterResponse{}, nil
}
