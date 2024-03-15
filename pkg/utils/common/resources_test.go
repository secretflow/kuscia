package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
)

func TestIsEmpty(t *testing.T) {
	var limitResource corev1.ResourceList
	assert.Equal(t, IsEmpty(limitResource), true, "IsEmpty() function cannot judge whether the data is empty. ")
	cpu := k8sresource.MustParse("100m")
	limitResource = corev1.ResourceList{}
	limitResource[corev1.ResourceCPU] = cpu
	assert.Equal(t, IsEmpty(limitResource), false, "IsEmpty() function cannot judge whether the data is not empty. ")
}

func TestSplitRSC(t *testing.T) {
	var input string = "1000"
	output, err := SplitRSC(input, 5)
	assert.Equal(t, output, "200", "SplitRSC() function cannot handle the k8sresource.DecimalSI. ")
	assert.Nil(t, err, "SplitRSC() function cannot handle the k8sresource.DecimalSI. ")

	inputSlice := [6]string{"100Pi", "200T", "300Gi", "400M", "500Ki", "0.5Pi"}
	stdOutputSlice := [6]string{"102400Ti", "100T", "76800Mi", "50M", "32000", "16384Gi"}
	for k, _ := range inputSlice {
		output, err = SplitRSC(inputSlice[k], 1<<k)
		assert.Equal(t, output, stdOutputSlice[k], "SplitRSC() function cannot handle the unit (Pi, Ti, Gi, Mi, Ki). ")
		assert.Nil(t, err, "SplitRSC() function cannot handle the unit (Pi, Ti, Gi, Mi, Ki). ")
	}

	input = "100KB"
	_, err = SplitRSC(input, 1<<5)
	assert.NotNil(t, err, "SplitRSC() function cannot handle the anomaly input. ")
}
