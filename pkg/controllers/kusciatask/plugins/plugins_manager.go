package plugins

import (
	"context"
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
}

type PluginManager struct {
	plugins []Plugin
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins: make([]Plugin, 0),
	}
}

func (pm *PluginManager) Register(p Plugin) {
	pm.plugins = append(pm.plugins, p)
}

func (pm *PluginManager) Permit(ctx context.Context, params interface{}) (bool, []error) {
	var errors []error
	for _, p := range pm.plugins {
		_, err := p.Permit(ctx, params)
		if err != nil {
			errors = append(errors, err)
			continue
		}
	}
	return len(errors) == 0, errors
}
