package dependencies

import (
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"

	"github.com/secretflow/kuscia/pkg/controllers/kusciatask/plugins"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
)

// Dependencies defines some parameter dependencies of functions.
type Dependencies struct {
	KubeClient       kubernetes.Interface
	KusciaClient     kusciaclientset.Interface
	TrgLister        kuscialistersv1alpha1.TaskResourceGroupLister
	NamespacesLister corelisters.NamespaceLister
	CdrLister        kuscialistersv1alpha1.ClusterDomainRouteLister
	PodsLister       corelisters.PodLister
	ServicesLister   corelisters.ServiceLister
	ConfigMapLister  corelisters.ConfigMapLister
	AppImagesLister  kuscialistersv1alpha1.AppImageLister
	Recorder         record.EventRecorder
	PluginManager    *plugins.PluginManager
}
