package reconcilers

import (
	"fmt"
	"net/http"

	"k8s.io/client-go/tools/record"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	opensearchPending     = "OpensearchPending"
	opensearchError       = "OpensearchError"
	opensearchAPIError    = "OpensearchAPIError"
	opensearchRefMismatch = "OpensearchRefMismatch"
	opensearchAPIUpdated  = "OpensearchAPIUpdated"
	passwordError         = "PasswordError"
	statusError           = "StatusUpdateError"
)

type ComponentReconciler func() (reconcile.Result, error)

type ReconcilerOptions struct {
	osClientTransport http.RoundTripper
	updateStatus      *bool
}

type ReconcilerOption func(*ReconcilerOptions)

func (o *ReconcilerOptions) apply(opts ...ReconcilerOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithOSClientTransport(transport http.RoundTripper) ReconcilerOption {
	return func(o *ReconcilerOptions) {
		o.osClientTransport = transport
	}
}

func WithUpdateStatus(update bool) ReconcilerOption {
	return func(o *ReconcilerOptions) {
		o.updateStatus = &update
	}
}

type ReconcilerContext struct {
	Volumes          []corev1.Volume
	VolumeMounts     []corev1.VolumeMount
	NodePoolHashes   []NodePoolHash
	DashboardsConfig map[string]string
	OpenSearchConfig map[string]string
	recorder         record.EventRecorder
	instance         *opensearchv1.OpenSearchCluster
}

type NodePoolHash struct {
	Component  string
	ConfigHash string
}

func NewReconcilerContext(recorder record.EventRecorder, instance *opensearchv1.OpenSearchCluster, nodepools []opensearchv1.NodePool) ReconcilerContext {
	var nodePoolHashes []NodePoolHash
	for _, nodepool := range nodepools {
		nodePoolHashes = append(nodePoolHashes, NodePoolHash{
			Component: nodepool.Component,
		})
	}
	return ReconcilerContext{
		NodePoolHashes:   nodePoolHashes,
		OpenSearchConfig: make(map[string]string),
		DashboardsConfig: make(map[string]string),
		recorder:         recorder,
		instance:         instance,
	}
}

func (c *ReconcilerContext) AddConfig(key string, value string) {
	_, exists := c.OpenSearchConfig[key]
	if exists {
		fmt.Printf("Warning: Config key '%s' already exists. Will be overwritten\n", key)
		c.recorder.Eventf(c.instance, "Warning", "ConfigDuplicateKey", "Config key '%s' already exists in opensearch config. Will be overwritten", key)
	}
	c.OpenSearchConfig[key] = value
}

func (c *ReconcilerContext) AddDashboardsConfig(key string, value string) {
	_, exists := c.DashboardsConfig[key]
	if exists {
		fmt.Printf("Warning: Dashboards Config key '%s' already exists. Will be overwritten\n", key)
		c.recorder.Eventf(c.instance, "Warning", "ConfigDuplicateKey", "Config key '%s' already exists in dashboards config. Will be overwritten", key)
	}
	c.DashboardsConfig[key] = value
}

// fetchNodePoolHash gets the hash of the config for a specific node pool
func (c *ReconcilerContext) fetchNodePoolHash(name string) (bool, NodePoolHash) {
	for _, config := range c.NodePoolHashes {
		if config.Component == name {
			return true, config
		}
	}
	return false, NodePoolHash{}
}

// replaceNodePoolHash updates the hash of the config for a specific node pool
func (c *ReconcilerContext) replaceNodePoolHash(newConfig NodePoolHash) {
	var configs []NodePoolHash
	for _, config := range c.NodePoolHashes {
		if config.Component == newConfig.Component {
			configs = append(configs, newConfig)
		} else {
			configs = append(configs, config)
		}
	}
	c.NodePoolHashes = configs
}

func UpdateComponentStatus(
	k8sClient k8s.K8sClient,
	cluster *opensearchv1.OpenSearchCluster,
	status *opensearchv1.ComponentStatus,
) error {
	if status != nil {
		return k8sClient.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(cluster), func(instance *opensearchv1.OpenSearchCluster) {
			found := false
			for idx, value := range instance.Status.ComponentsStatus {
				if value.Component == status.Component {
					instance.Status.ComponentsStatus[idx] = *status
					found = true
					break
				}
			}
			if !found {
				instance.Status.ComponentsStatus = append(instance.Status.ComponentsStatus, *status)
			}
		})
	}
	return nil
}
