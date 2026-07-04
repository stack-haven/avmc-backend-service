package biz

import (
	"testing"

	pb "backend-service/api/core/service/v1"
)

func TestValidateParameterDefinition(t *testing.T) {
	tests := []struct {
		name    string
		item    *pb.ParameterDefinition
		wantErr bool
	}{
		{
			name: "valid integer",
			item: &pb.ParameterDefinition{
				Key: "system.page_size", Name: "Page size",
				ValueType: pb.ParameterValueType_PARAMETER_VALUE_TYPE_INTEGER, DefaultValue: "20",
			},
		},
		{
			name: "invalid key",
			item: &pb.ParameterDefinition{
				Key: "PageSize", Name: "Page size",
				ValueType: pb.ParameterValueType_PARAMETER_VALUE_TYPE_INTEGER, DefaultValue: "20",
			},
			wantErr: true,
		},
		{
			name: "secret rejected",
			item: &pb.ParameterDefinition{
				Key: "integration.api_key", Name: "API key",
				ValueType: pb.ParameterValueType_PARAMETER_VALUE_TYPE_STRING, DefaultValue: "secret",
			},
			wantErr: true,
		},
		{
			name: "invalid boolean",
			item: &pb.ParameterDefinition{
				Key: "feature.enabled", Name: "Feature enabled",
				ValueType: pb.ParameterValueType_PARAMETER_VALUE_TYPE_BOOLEAN, DefaultValue: "yes",
			},
			wantErr: true,
		},
		{
			name: "invalid json",
			item: &pb.ParameterDefinition{
				Key: "feature.options", Name: "Feature options",
				ValueType: pb.ParameterValueType_PARAMETER_VALUE_TYPE_JSON, DefaultValue: "{",
			},
			wantErr: true,
		},
		{
			name: "token ttl is not a secret",
			item: &pb.ParameterDefinition{
				Key: "auth.access_token_ttl", Name: "Token TTL",
				ValueType: pb.ParameterValueType_PARAMETER_VALUE_TYPE_INTEGER, DefaultValue: "3600",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateParameterDefinition(test.item)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateParameterDefinition() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}
