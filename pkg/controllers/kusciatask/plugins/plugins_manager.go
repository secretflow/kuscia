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

func (pm *PluginManager) Permit(ctx context.Context, pluginType PluginType, params interface{}) (bool, error) {
	if p, exists := pm.plugins[pluginType]; exists {
		return p.Permit(ctx, params)
	}
	return false, fmt.Errorf("plugin %s not registered", pluginType)
}

