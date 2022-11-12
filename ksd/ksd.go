package ksd

import (
	"fmt"
	"github.com/danthegoodman1/PSQLGateway/gologger"
	v1 "k8s.io/api/core/v1"
	"time"

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
		kubeClient      *kubernetes.Clientset
		informerFactory informers.SharedInformerFactory
		podInformer     cache.SharedIndexInformer
		stopChan        chan struct{}
	}
)

// NewPodKSD creates a new endpoint watcher that listens for added, deleted, and updated endpoints.
func NewPodKSD(namespace, labelSelector string, addFunc, deleteFunc func(obj *v1.Pod), updateFunc func(oldObj, newObj *v1.Pod)) (*KSD, error) {
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

	podInformer := informerFactory.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			logger.Debug().Interface("obj", obj).Str("informer", "pod").Msg("got add event")
			addFunc(obj.(*v1.Pod))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			logger.Debug().Interface("old_obj", oldObj).Interface("new_obj", newObj).Str("informer", "pod").Msg("got update event")
			updateFunc(oldObj.(*v1.Pod), newObj.(*v1.Pod))
		},
		DeleteFunc: func(obj interface{}) {
			logger.Debug().Interface("obj", obj).Str("informer", "pod").Msg("got delete event")
			deleteFunc(obj.(*v1.Pod))
		},
	})
	stopChan := make(chan struct{})
	go podInformer.Run(stopChan)

	ksd := &KSD{
		kubeClient:      client,
		informerFactory: informerFactory,
		podInformer:     podInformer,
		stopChan:        stopChan,
	}

	return ksd, nil
}

func (k *KSD) Stop() {
	close(k.stopChan)
}
