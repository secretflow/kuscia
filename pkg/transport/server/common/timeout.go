package common

import "time"

const (
	ParamTimeout   = "timeout"
	DefaultTimeout = time.Second * 120
	MinTimeout     = time.Second
	MaxTimeout     = time.Second * 300
)
