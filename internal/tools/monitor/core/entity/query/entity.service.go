// Copyright (c) 2021 Terminus, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package entity

import (
	"context"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/erda-project/erda-proto-go/oap/entity/pb"
	"github.com/erda-project/erda/internal/tools/monitor/core/entity"
	"github.com/erda-project/erda/internal/tools/monitor/core/entity/storage"
	"github.com/erda-project/erda/pkg/common/errors"
)

type entityService struct {
	p       *provider
	storage storage.Storage
}

func (s *entityService) SetEntity(ctx context.Context, req *pb.SetEntityRequest) (*pb.SetEntityResponse, error) {
	if req.Data == nil {
		return nil, errors.NewMissingParameterError("body")
	}
	err := s.storage.SetEntity(ctx, convert(req.Data))
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return &pb.SetEntityResponse{
		Data: req.Data.Id,
	}, nil
}

func (s *entityService) RemoveEntity(ctx context.Context, req *pb.RemoveEntityRequest) (*pb.RemoveEntityResponse, error) {
	ok, err := s.storage.RemoveEntity(ctx, req.Type, req.Key)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return &pb.RemoveEntityResponse{
		Ok: ok,
	}, nil
}

func (s *entityService) GetEntity(ctx context.Context, req *pb.GetEntityRequest) (*pb.GetEntityResponse, error) {
	entity, err := s.storage.GetEntity(ctx, req.Type, req.Key)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return &pb.GetEntityResponse{
		Data: convertToPb(entity),
	}, nil
}

func (s *entityService) ListEntities(ctx context.Context, req *pb.ListEntitiesRequest) (*pb.ListEntitiesResponse, error) {
	list, total, err := s.storage.ListEntities(ctx, &storage.ListOptions{
		Type:                  req.Type,
		Labels:                req.Labels,
		Limit:                 int(req.Limit),
		UpdateTimeUnixNanoMax: req.UpdateTimeUnixNanoMax,
		UpdateTimeUnixNanoMin: req.UpdateTimeUnixNanoMin,
		Debug:                 req.Debug,
	})
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	var pbList []*pb.Entity
	for i := range list {
		pbList = append(pbList, convertToPb(list[i]))
	}
	return &pb.ListEntitiesResponse{
		Data: &pb.EntityList{
			List:  pbList,
			Total: total,
		},
	}, nil
}

func convert(input *pb.Entity) *entity.Entity {
	values := make(map[string]string)
	for k, v := range input.Values {
		values[k] = v.String()
	}
	return &entity.Entity{
		ID:                 input.Id,
		Type:               input.Type,
		Key:                input.Key,
		Values:             values,
		Labels:             input.Labels,
		CreateTimeUnixNano: input.CreateTimeUnixNano,
		UpdateTimeUnixNano: input.UpdateTimeUnixNano,
	}
}

func convertToPb(input *entity.Entity) *pb.Entity {
	values := make(map[string]*structpb.Value)
	for k, v := range input.Values {
		values[k] = structpb.NewStringValue(v)
	}
	return &pb.Entity{
		Id:                 input.ID,
		Type:               input.Type,
		Key:                input.Key,
		Values:             values,
		Labels:             input.Labels,
		CreateTimeUnixNano: input.CreateTimeUnixNano,
		UpdateTimeUnixNano: input.UpdateTimeUnixNano,
	}
}
