package configuration

import (
	"fmt"

	"github.com/secretflow/kuscia/pkg/confmanager/interceptor"
	"github.com/secretflow/kuscia/pkg/web/api"
)

func validateTLSOu(context *api.BizContext) error {
	tlsCert := interceptor.TLSCertFromGinContext(context.Context)
	if tlsCert == nil || len(tlsCert.OrganizationalUnit) == 0 {
		return fmt.Errorf("require client tls ou")
	}
	return nil
}
