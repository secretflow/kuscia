package plugins

import (
	"context"
	"fmt"
)

type PluginType string

const (
	PluginTypeResourceCheck PluginType = "resource-check"
	PluginTypeCDRCheck      PluginType = "cdr-check"
)

type ResourceRequest struct {
	DomainName string
	CpuReq     int64
	MemReq     int64
}

type CompositeRequest struct {
	ResourceReq ResourceRequest
	CDRReq      []string
}

type Plugin interface {
	Permit(ctx context.Context, params interface{}) (bool, error)
	Type() PluginType
}

type PluginManager struct {
	plugins map[PluginType]Plugin
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins: make(map[PluginType]Plugin),
	}
}

func (pm *PluginManager) Register(p Plugin) {
	pm.plugins[p.Type()] = p
}

func (pm *PluginManager) Permit(ctx context.Context, params interface{}) (bool, []error) {
	var errors []error
	for _, p := range pm.plugins {
		passed, err := p.Permit(ctx, params)
		if err != nil {
			errors = append(errors, fmt.Errorf("%s plugin: %v", p.Type(), err))
			continue
		}
		if !passed {
			errors = append(errors, fmt.Errorf("%s check failed", p.Type()))
		}
	}
	return len(errors) == 0, errors
}
