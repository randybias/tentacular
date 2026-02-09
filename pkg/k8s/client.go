package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/randyb/pipedreamer2/pkg/builder"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps K8s client-go for pipedreamer operations.
type Client struct {
	clientset *kubernetes.Clientset
	dynamic   dynamic.Interface
	config    *rest.Config
}

// NewClient creates a K8s client from kubeconfig or in-cluster config.
func NewClient() (*Client, error) {
	config, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating clientset: %w", err)
	}

	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	return &Client{
		clientset: clientset,
		dynamic:   dyn,
		config:    config,
	}, nil
}

func loadConfig() (*rest.Config, error) {
	// Try in-cluster first
	if config, err := rest.InClusterConfig(); err == nil {
		return config, nil
	}

	// Fall back to kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

const k8sTimeout = 30 * time.Second

// Apply applies a set of K8s manifests to the cluster.
func (c *Client) Apply(namespace string, manifests []builder.Manifest) error {
	ctx, cancel := context.WithTimeout(context.Background(), k8sTimeout)
	defer cancel()
	decSerializer := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	for _, m := range manifests {
		obj := &unstructured.Unstructured{}
		_, gvk, err := decSerializer.Decode([]byte(m.Content), nil, obj)
		if err != nil {
			return fmt.Errorf("decoding %s %s: %w", m.Kind, m.Name, err)
		}

		mapping, err := c.findResource(gvk.Group, gvk.Version, gvk.Kind)
		if err != nil {
			return fmt.Errorf("finding resource mapping for %s: %w", gvk.Kind, err)
		}

		resource := c.dynamic.Resource(mapping).Namespace(namespace)

		existing, err := resource.Get(ctx, m.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = resource.Create(ctx, obj, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("creating %s %s: %w", m.Kind, m.Name, err)
			}
			fmt.Printf("  created %s/%s\n", m.Kind, m.Name)
		} else if err != nil {
			return fmt.Errorf("checking %s %s: %w", m.Kind, m.Name, err)
		} else {
			obj.SetResourceVersion(existing.GetResourceVersion())
			_, err = resource.Update(ctx, obj, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("updating %s %s: %w", m.Kind, m.Name, err)
			}
			fmt.Printf("  updated %s/%s\n", m.Kind, m.Name)
		}
	}

	return nil
}

func (c *Client) findResource(group, version, kind string) (schema.GroupVersionResource, error) {
	resourceMap := map[string]string{
		"Deployment": "deployments",
		"Service":    "services",
		"ConfigMap":  "configmaps",
		"Secret":     "secrets",
	}

	resource, ok := resourceMap[kind]
	if !ok {
		return schema.GroupVersionResource{}, fmt.Errorf("unknown kind: %s", kind)
	}

	return schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}, nil
}

// DeploymentStatus represents the status of a deployed workflow.
type DeploymentStatus struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Ready     bool   `json:"ready"`
	Replicas  int32  `json:"replicas"`
	Available int32  `json:"available"`
	Message   string `json:"message,omitempty"`
}

func (s *DeploymentStatus) JSON() string {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "failed to marshal status: %s"}`, err)
	}
	return string(data)
}

func (s *DeploymentStatus) Text() string {
	status := "not ready"
	if s.Ready {
		status = "ready"
	}
	return fmt.Sprintf("Workflow: %s\nNamespace: %s\nStatus: %s\nReplicas: %d/%d",
		s.Name, s.Namespace, status, s.Available, s.Replicas)
}

// GetStatus returns the deployment status of a workflow.
func (c *Client) GetStatus(namespace, name string) (*DeploymentStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), k8sTimeout)
	defer cancel()
	dep, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting deployment: %w", err)
	}

	replicas := int32(1)
	if dep.Spec.Replicas != nil {
		replicas = *dep.Spec.Replicas
	}

	return &DeploymentStatus{
		Name:      name,
		Namespace: namespace,
		Ready:     dep.Status.ReadyReplicas == replicas,
		Replicas:  replicas,
		Available: dep.Status.AvailableReplicas,
	}, nil
}
