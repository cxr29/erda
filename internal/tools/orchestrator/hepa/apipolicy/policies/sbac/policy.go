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

package sbac

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/erda-project/erda/internal/tools/orchestrator/hepa/apipolicy"
	"github.com/erda-project/erda/internal/tools/orchestrator/hepa/kong"
	kongDto "github.com/erda-project/erda/internal/tools/orchestrator/hepa/kong/dto"
	"github.com/erda-project/erda/internal/tools/orchestrator/hepa/repository/orm"
	db "github.com/erda-project/erda/internal/tools/orchestrator/hepa/repository/service"
)

const (
	// Name "sbac" is ServerBasedAccessControl
	Name = "sbac"
)

func init() {
	if err := apipolicy.RegisterPolicyEngine(Name, new(Policy)); err != nil {
		panic(err)
	}
}

type Policy struct {
	apipolicy.BasePolicy
}

func (policy Policy) CreateDefaultConfig(ctx map[string]interface{}) apipolicy.PolicyDto {
	return new(PolicyDto)
}

func (policy Policy) UnmarshalConfig(config []byte) (apipolicy.PolicyDto, error, string) {
	var policyDto PolicyDto
	if err := json.Unmarshal(config, &policyDto); err != nil {
		return nil, errors.Wrapf(err, "failed to Unmarshal config: %s", string(config)), "Invalid config"
	}
	if err := policyDto.IsValidDto(); err != nil {
		return nil, errors.Wrap(err, "Invalid config"), "Invalid config"
	}
	return &policyDto, nil, ""
}

func (policy Policy) buildPluginReq(dto *PolicyDto) *kongDto.KongPluginReqDto {
	return dto.ToPluginReqDto()
}

func (policy Policy) ParseConfig(dto apipolicy.PolicyDto, ctx map[string]interface{}) (apipolicy.PolicyConfig, error) {
	l := logrus.WithField("pluginName", Name).WithField("func", "ParseConfig")
	l.Infof("dto: %+v", dto)
	res := apipolicy.PolicyConfig{}
	policyDto, ok := dto.(*PolicyDto)
	if !ok {
		return res, errors.Errorf("invalid config:%+v", dto)
	}
	adapter, ok := ctx[apipolicy.CTX_KONG_ADAPTER].(kong.KongAdapter)
	if !ok {
		return res, errors.Errorf("failed to get identify with %s: %+v", apipolicy.CTX_KONG_ADAPTER, ctx)
	}
	kongVersion, err := adapter.GetVersion()
	if err != nil {
		return res, errors.Wrap(err, "failed to retrieve Kong version")
	}
	if !strings.HasPrefix(kongVersion, "2.") {
		return res, errors.Errorf("the plugin %s is not supportted on the Kong version %s", Name, kongVersion)
	}
	zone, ok := ctx[apipolicy.CTX_ZONE].(*orm.GatewayZone)
	if !ok {
		return res, errors.Errorf("failed to get identify with %s: %+v", apipolicy.CTX_ZONE, ctx)
	}
	policyDb, _ := db.NewGatewayPolicyServiceImpl()
	exist, err := policyDb.GetByAny(&orm.GatewayPolicy{
		ZoneId:     zone.Id,
		PluginName: Name,
	})
	if err != nil {
		return res, err
	}
	if !policyDto.Switch {
		if exist != nil {
			err = adapter.RemovePlugin(exist.PluginId)
			if err != nil {
				return res, err
			}
			_ = policyDb.DeleteById(exist.Id)
			res.KongPolicyChange = true
		}
		return res, nil
	}
	req := policy.buildPluginReq(policyDto)
	if exist != nil {
		req.Id = exist.PluginId
		resp, err := adapter.CreateOrUpdatePluginById(req)
		if err != nil {
			return res, err
		}
		configByte, err := json.Marshal(resp.Config)
		if err != nil {
			return res, err
		}
		exist.Config = configByte
		err = policyDb.Update(exist)
		if err != nil {
			return res, err
		}
	} else {
		resp, err := adapter.AddPlugin(req)
		if err != nil {
			return res, err
		}
		configByte, err := json.Marshal(resp.Config)
		if err != nil {
			return res, err
		}
		policyDao := &orm.GatewayPolicy{
			ZoneId:     zone.Id,
			PluginName: Name,
			Category:   "safety",
			PluginId:   resp.Id,
			Config:     configByte,
			Enabled:    1,
		}
		err = policyDb.Insert(policyDao)
		if err != nil {
			return res, err
		}
		res.KongPolicyChange = true
	}
	return res, nil
}
