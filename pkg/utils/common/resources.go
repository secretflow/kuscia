package common

import (
	"errors"
	"fmt"
	"reflect"

	k8sresource "k8s.io/apimachinery/pkg/api/resource"
)

// IsEmpty will judge whether data is empty
func IsEmpty(v interface{}) bool {
	return reflect.DeepEqual(v, reflect.Zero(reflect.TypeOf(v)).Interface())
}

// SplitRSC will split the resources into N parts
func SplitRSC(rsc string, n int) (string, error) {
	quantity, err := k8sresource.ParseQuantity(rsc)
	if err != nil {
		return "", errors.New("failed to parse resource quantity: " + err.Error())
	}
	unit := quantity.Format
	if unit == k8sresource.DecimalSI {
		quantity.SetMilli(quantity.MilliValue() / int64(n))
		return quantity.String(), nil
	} else {
		bytes := quantity.Value()
		bytesPerPart := bytes / int64(n)
		var result string
		switch {
		case bytesPerPart >= 1<<60:
			result = fmt.Sprintf("%.0fPi", float64(bytesPerPart)/(1<<50))
		case bytesPerPart >= 1<<50:
			result = fmt.Sprintf("%.0fTi", float64(bytesPerPart)/(1<<40))
		case bytesPerPart >= 1<<40:
			result = fmt.Sprintf("%.0fGi", float64(bytesPerPart)/(1<<30))
		case bytesPerPart >= 1<<30:
			result = fmt.Sprintf("%.0fMi", float64(bytesPerPart)/(1<<20))
		case bytesPerPart >= 1<<20:
			result = fmt.Sprintf("%.0fKi", float64(bytesPerPart)/(1<<10))
		default:
			quantity.Set(bytesPerPart)
			result = quantity.String()
		}
		return result, nil
	}
}
