package plugins

import (
	"context"
)

type ResourceRequest struct {
	DomainName string
	CpuReq     int64
	MemReq     int64
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
