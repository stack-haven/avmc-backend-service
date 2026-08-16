package service

import (
	"context"
	"strconv"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
)

type ParameterServiceService struct {
	pb.UnimplementedParameterServiceServer
	uc  *biz.ParameterUsecase
	log *log.Helper
}

func NewParameterServiceService(uc *biz.ParameterUsecase, logger log.Logger) *ParameterServiceService {
	return &ParameterServiceService{uc: uc, log: log.NewHelper(logger)}
}

func (s *ParameterServiceService) ListParameterDefinitions(ctx context.Context, req *pb.ListParameterDefinitionsRequest) (*pb.ListParameterDefinitionsResponse, error) {
	items, total, err := s.uc.ListDefinitions(ctx, req)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListParameterDefinitionsResponse{Items: items, Total: total}
	offset, err := strconv.Atoi(req.GetPageToken())
	if err != nil {
		offset = 0
	}
	if offset+len(items) < int(total) {
		resp.NextPageToken = strconv.Itoa(offset + len(items))
	}
	return resp, nil
}

func (s *ParameterServiceService) GetParameterDefinition(ctx context.Context, req *pb.GetParameterDefinitionRequest) (*pb.GetParameterDefinitionResponse, error) {
	item, err := s.uc.GetDefinition(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pb.GetParameterDefinitionResponse{Parameter: item}, nil
}

func (s *ParameterServiceService) CreateParameterDefinition(ctx context.Context, req *pb.CreateParameterDefinitionRequest) (*pb.CreateParameterDefinitionResponse, error) {
	item, err := s.uc.CreateDefinition(ctx, req.GetParameter())
	if err != nil {
		return nil, err
	}
	return &pb.CreateParameterDefinitionResponse{Parameter: item}, nil
}

func (s *ParameterServiceService) UpdateParameterDefinition(ctx context.Context, req *pb.UpdateParameterDefinitionRequest) (*pb.UpdateParameterDefinitionResponse, error) {
	req.Parameter.Id = req.GetId()
	item, err := s.uc.UpdateDefinition(ctx, req.GetParameter())
	if err != nil {
		return nil, err
	}
	return &pb.UpdateParameterDefinitionResponse{Parameter: item}, nil
}

func (s *ParameterServiceService) DeleteParameterDefinition(ctx context.Context, req *pb.DeleteParameterDefinitionRequest) (*pb.DeleteParameterDefinitionResponse, error) {
	if err := s.uc.DeleteDefinition(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pb.DeleteParameterDefinitionResponse{}, nil
}

func (s *ParameterServiceService) ListCurrentTenantParameters(ctx context.Context, req *pb.ListCurrentTenantParametersRequest) (*pb.ListCurrentTenantParametersResponse, error) {
	items, err := s.uc.ListCurrent(ctx, req.GetKey())
	if err != nil {
		return nil, err
	}
	return &pb.ListCurrentTenantParametersResponse{Items: items}, nil
}

func (s *ParameterServiceService) SetCurrentTenantParameter(ctx context.Context, req *pb.SetCurrentTenantParameterRequest) (*pb.SetCurrentTenantParameterResponse, error) {
	item, err := s.uc.SetCurrent(ctx, req.GetKey(), req.GetValue())
	if err != nil {
		return nil, err
	}
	return &pb.SetCurrentTenantParameterResponse{Parameter: item}, nil
}

func (s *ParameterServiceService) ResetCurrentTenantParameter(ctx context.Context, req *pb.ResetCurrentTenantParameterRequest) (*pb.ResetCurrentTenantParameterResponse, error) {
	if err := s.uc.ResetCurrent(ctx, req.GetKey()); err != nil {
		return nil, err
	}
	return &pb.ResetCurrentTenantParameterResponse{}, nil
}

func (s *ParameterServiceService) ListTenantParameters(ctx context.Context, req *pb.ListTenantParametersRequest) (*pb.ListTenantParametersResponse, error) {
	items, err := s.uc.ListTenant(ctx, req.GetTenantId(), req.GetKey())
	if err != nil {
		return nil, err
	}
	return &pb.ListTenantParametersResponse{Items: items}, nil
}

func (s *ParameterServiceService) SetTenantParameter(ctx context.Context, req *pb.SetTenantParameterRequest) (*pb.SetTenantParameterResponse, error) {
	item, err := s.uc.SetTenant(ctx, req.GetTenantId(), req.GetKey(), req.GetValue())
	if err != nil {
		return nil, err
	}
	return &pb.SetTenantParameterResponse{Parameter: item}, nil
}

func (s *ParameterServiceService) ResetTenantParameter(ctx context.Context, req *pb.ResetTenantParameterRequest) (*pb.ResetTenantParameterResponse, error) {
	if err := s.uc.ResetTenant(ctx, req.GetTenantId(), req.GetKey()); err != nil {
		return nil, err
	}
	return &pb.ResetTenantParameterResponse{}, nil
}
