package utils

import (
	"fmt"
	"strings"

	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

func GetPrefixIfPresent(endpoint v1alpha1.DomainEndpoint) string {
	if len(endpoint.Ports) > 0 {
		return strings.TrimSuffix(endpoint.Ports[0].PathPrefix, "/")
	}
	return ""
}

func GetHandshakePathSuffix() string {
	return "/handshake"
}

func GetHandshakePathOfEndpoint(endpoint v1alpha1.DomainEndpoint) string {
	return fmt.Sprintf("%s%s", GetPrefixIfPresent(endpoint), GetHandshakePathSuffix())
}

func GetHandshakePathOfPrefix(pathPrefix string) string {
	return fmt.Sprintf("%s%s", strings.TrimSuffix(pathPrefix, "/"), GetHandshakePathSuffix())
}
