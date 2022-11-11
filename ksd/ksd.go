package ksd

import (
	"fmt"
	"github.com/danthegoodman1/PSQLGateway/gologger"
	"time"

	v1 "k8s.io/api/discovery/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

var (
	logger = gologger.NewLogger()
)

type (
	KSD struct {
		kubeClient       *kubernetes.Clientset
		informerFactory  informers.SharedInformerFactory
		endpointInformer cache.SharedIndexInformer
		stopChan         chan struct{}
	}
)

// NewEndpointKSD creates a new endpoint watcher that listens for added, deleted, and updated endpoints.
func NewEndpointKSD(namespace, labelSelector string, addFunc, deleteFunc func(obj *v1.Endpoint), updateFunc func(oldObj, newObj *v1.Endpoint)) (*KSD, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("error in rest.InClusterConfig: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error in kubernetes.NewForConfig: %w", err)
	}
	logger.Debug().Msg("starting informer factory")

	// TODO: Make time configurable
	resyncTime := 30 * time.Second
	informerFactory := informers.NewSharedInformerFactoryWithOptions(client, resyncTime, informers.WithNamespace(namespace), informers.WithTweakListOptions(func(options *v12.ListOptions) {
		options.LabelSelector = labelSelector
	}))

	endpointInformer := informerFactory.Core().V1().Endpoints().Informer()
	endpointInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			logger.Debug().Interface("obj", obj).Str("informer", "endpoint").Msg("got add event")
			addFunc(obj.(*v1.Endpoint))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			logger.Debug().Interface("old_obj", oldObj).Interface("new_obj", newObj).Str("informer", "endpoint").Msg("got update event")
			updateFunc(oldObj.(*v1.Endpoint), newObj.(*v1.Endpoint))
		},
		DeleteFunc: func(obj interface{}) {
			logger.Debug().Interface("obj", obj).Str("informer", "endpoint").Msg("got delete event")
			deleteFunc(obj.(*v1.Endpoint))
		},
	})
	stopChan := make(chan struct{})
	go endpointInformer.Run(stopChan)

	ksd := &KSD{
		kubeClient:       client,
		informerFactory:  informerFactory,
		endpointInformer: endpointInformer,
		stopChan:         stopChan,
	}

	return ksd, nil
}

func (k *KSD) Stop() {
	close(k.stopChan)
}
