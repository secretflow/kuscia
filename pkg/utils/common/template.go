package common

import (
	"bytes"
	"os"
	"text/template"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func RenderConfig(configPathTmpl, configPath string, s interface{}) error {
	configTmpl, err := template.ParseFiles(configPathTmpl)
	if err != nil {
		return err
	}
	var configContent bytes.Buffer
	if err := configTmpl.Execute(&configContent, s); err != nil {
		return err
	}

	f, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(configContent.String())
	if err != nil {
		return err
	}
	return nil
}

func RenderRuntimeObject(configPathTmpl string, object runtime.Object, input interface{}) error {
	template, err := template.ParseFiles(configPathTmpl)
	if err != nil {
		return err
	}

	var buffer bytes.Buffer
	if err := template.Execute(&buffer, input); err != nil {
		return err
	}

	if _, _, err = Decode(buffer.Bytes(), nil, object); err != nil {
		return err
	}
	return nil
}

var Decode func(data []byte, defaults *schema.GroupVersionKind, into runtime.Object) (runtime.Object, *schema.GroupVersionKind, error)

func init() {
	sch := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(sch)
	_ = apiextv1beta1.AddToScheme(sch)
	_ = appsv1.AddToScheme(sch)
	_ = corev1.AddToScheme(sch)
	Decode = serializer.NewCodecFactory(sch).UniversalDeserializer().Decode
}
