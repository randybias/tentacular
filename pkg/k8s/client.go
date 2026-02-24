package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"


	"github.com/randybias/tentacular/pkg/builder"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/apimachinery/pkg/types"
)

// Client wraps K8s client-go for tentacular operations.
type Client struct {
	clientset       *kubernetes.Clientset
	dynamic         dynamic.Interface
	config          *rest.Config
	lastApplyUpdate bool // true if the last Apply/ApplyWithStatus had any updates (vs all creates)
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

// NewClientWithContext creates a K8s client using an explicit kubeconfig context.
func NewClientWithContext(contextName string) (*Client, error) {
	config, err := loadConfigWithContext(contextName)
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig for context %s: %w", contextName, err)
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

func loadConfigWithContext(contextName string) (*rest.Config, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{CurrentContext: contextName},
	).ClientConfig()
}

func loadConfigFromFile(kubeconfigPath, contextName string) (*rest.Config, error) {
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		overrides,
	).ClientConfig()
}

// NewClientFromConfig creates a K8s client using an explicit kubeconfig file path
// and optional context name. This is used when environment config specifies a
// kubeconfig file rather than relying on KUBECONFIG env var.
func NewClientFromConfig(kubeconfigPath, contextName string) (*Client, error) {
	config, err := loadConfigFromFile(kubeconfigPath, contextName)
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig %s: %w", kubeconfigPath, err)
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

// WaitForReady polls until a deployment's ReadyReplicas equals Replicas or the context expires.
func (c *Client) WaitForReady(ctx context.Context, namespace, name string) error {
	for {
		dep, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("getting deployment %s: %w", name, err)
		}

		replicas := int32(1)
		if dep.Spec.Replicas != nil {
			replicas = *dep.Spec.Replicas
		}

		if dep.Status.ReadyReplicas >= replicas {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for deployment %s to become ready (ready: %d/%d)", name, dep.Status.ReadyReplicas, replicas)
		case <-time.After(2 * time.Second):
			// poll again
		}
	}
}

const k8sTimeout = 30 * time.Second

// Apply applies a set of K8s manifests to the cluster.
// Status messages are written to os.Stdout. Use ApplyWithStatus to control output destination.
func (c *Client) Apply(namespace string, manifests []builder.Manifest) error {
	return c.ApplyWithStatus(os.Stdout, namespace, manifests)
}

// ApplyWithStatus applies K8s manifests and writes status messages to w.
// Tracks whether any resources were updated (vs all created) for rollout restart decisions.
func (c *Client) ApplyWithStatus(w io.Writer, namespace string, manifests []builder.Manifest) error {
	c.lastApplyUpdate = false
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
			fmt.Fprintf(w, "  created %s/%s\n", m.Kind, m.Name)
		} else if err != nil {
			return fmt.Errorf("checking %s %s: %w", m.Kind, m.Name, err)
		} else {
			obj.SetResourceVersion(existing.GetResourceVersion())
			_, err = resource.Update(ctx, obj, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("updating %s %s: %w", m.Kind, m.Name, err)
			}
			fmt.Fprintf(w, "  updated %s/%s\n", m.Kind, m.Name)
			c.lastApplyUpdate = true
		}
	}

	return nil
}

// LastApplyHadUpdates returns true if the most recent Apply/ApplyWithStatus call
// updated any existing resources (as opposed to creating all new ones).
func (c *Client) LastApplyHadUpdates() bool {
	return c.lastApplyUpdate
}

func (c *Client) findResource(group, version, kind string) (schema.GroupVersionResource, error) {
	resourceMap := map[string]string{
		"Deployment":    "deployments",
		"Service":       "services",
		"ConfigMap":     "configmaps",
		"Secret":        "secrets",
		"CronJob":       "cronjobs",
		"NetworkPolicy": "networkpolicies",
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

// WorkflowInfo represents a deployed workflow in a listing.
type WorkflowInfo struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	Version   string    `json:"version"`
	Ready     bool      `json:"ready"`
	Replicas  int32     `json:"replicas"`
	Available int32     `json:"available"`
	Image     string    `json:"image"`
	Created   time.Time `json:"created"`
}

// PodStatus represents the status of a single pod.
type PodStatus struct {
	Name  string `json:"name"`
	Phase string `json:"phase"`
	Ready bool   `json:"ready"`
	IP    string `json:"ip"`
}

// DetailedStatus extends DeploymentStatus with additional cluster details.
type DetailedStatus struct {
	DeploymentStatus
	Image          string            `json:"image"`
	RuntimeClass   string            `json:"runtimeClass,omitempty"`
	ResourceLimits map[string]string `json:"resourceLimits,omitempty"`
	ServiceEndpoint string           `json:"serviceEndpoint,omitempty"`
	Pods           []PodStatus       `json:"pods,omitempty"`
	Events         []string          `json:"events,omitempty"`
}

func (s *DetailedStatus) JSON() string {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "failed to marshal status: %s"}`, err)
	}
	return string(data)
}

func (s *DetailedStatus) Text() string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "Workflow: %s\n", s.Name)
	fmt.Fprintf(&b, "Namespace: %s\n", s.Namespace)
	status := "not ready"
	if s.Ready {
		status = "ready"
	}
	fmt.Fprintf(&b, "Status: %s\n", status)
	fmt.Fprintf(&b, "Replicas: %d/%d\n", s.Available, s.Replicas)
	fmt.Fprintf(&b, "Image: %s\n", s.Image)
	if s.RuntimeClass != "" {
		fmt.Fprintf(&b, "Runtime Class: %s\n", s.RuntimeClass)
	}
	if len(s.ResourceLimits) > 0 {
		fmt.Fprintf(&b, "Resource Limits:\n")
		for k, v := range s.ResourceLimits {
			fmt.Fprintf(&b, "  %s: %s\n", k, v)
		}
	}
	if s.ServiceEndpoint != "" {
		fmt.Fprintf(&b, "Service: %s\n", s.ServiceEndpoint)
	}
	if len(s.Pods) > 0 {
		fmt.Fprintf(&b, "Pods:\n")
		for _, p := range s.Pods {
			readyStr := "not ready"
			if p.Ready {
				readyStr = "ready"
			}
			fmt.Fprintf(&b, "  %s  %s  %s  %s\n", p.Name, p.Phase, readyStr, p.IP)
		}
	}
	if len(s.Events) > 0 {
		fmt.Fprintf(&b, "Recent Events:\n")
		for _, e := range s.Events {
			fmt.Fprintf(&b, "  %s\n", e)
		}
	}
	return b.String()
}

// DeleteResources removes the Service, Deployment, and Secret for a workflow.
// Returns the list of deleted resource descriptions. NotFound errors are silently skipped.
func (c *Client) DeleteResources(namespace, name string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), k8sTimeout)
	defer cancel()

	var deleted []string
	propagation := metav1.DeletePropagationForeground

	// Delete Service
	err := c.clientset.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
	if errors.IsNotFound(err) {
		// skip
	} else if err != nil {
		return deleted, fmt.Errorf("deleting service %s: %w", name, err)
	} else {
		deleted = append(deleted, "Service/"+name)
	}

	// Delete Deployment
	err = c.clientset.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
	if errors.IsNotFound(err) {
		// skip
	} else if err != nil {
		return deleted, fmt.Errorf("deleting deployment %s: %w", name, err)
	} else {
		deleted = append(deleted, "Deployment/"+name)
	}

	// Delete Secret
	secretName := name + "-secrets"
	err = c.clientset.CoreV1().Secrets(namespace).Delete(ctx, secretName, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
	if errors.IsNotFound(err) {
		// skip
	} else if err != nil {
		return deleted, fmt.Errorf("deleting secret %s: %w", secretName, err)
	} else {
		deleted = append(deleted, "Secret/"+secretName)
	}

	// Delete ConfigMap (code bundle)
	configMapName := name + "-code"
	err = c.clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, configMapName, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
	if errors.IsNotFound(err) {
		// skip
	} else if err != nil {
		return deleted, fmt.Errorf("deleting configmap %s: %w", configMapName, err)
	} else {
		deleted = append(deleted, "ConfigMap/"+configMapName)
	}

	// Delete CronJobs by label selector
	labelSelector := fmt.Sprintf("app.kubernetes.io/name=%s,app.kubernetes.io/managed-by=tentacular", name)
	cronJobs, err := c.clientset.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return deleted, fmt.Errorf("listing cronjobs for %s: %w", name, err)
	}
	for _, cj := range cronJobs.Items {
		err = c.clientset.BatchV1().CronJobs(namespace).Delete(ctx, cj.Name, metav1.DeleteOptions{
			PropagationPolicy: &propagation,
		})
		if errors.IsNotFound(err) {
			// skip
		} else if err != nil {
			return deleted, fmt.Errorf("deleting cronjob %s: %w", cj.Name, err)
		} else {
			deleted = append(deleted, "CronJob/"+cj.Name)
		}
	}

	return deleted, nil
}

// ListWorkflows returns all deployments managed by tentacular in the given namespace.
func (c *Client) ListWorkflows(namespace string) ([]WorkflowInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), k8sTimeout)
	defer cancel()

	deps, err := c.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=tentacular",
	})
	if err != nil {
		return nil, fmt.Errorf("listing deployments: %w", err)
	}

	var workflows []WorkflowInfo
	for _, dep := range deps.Items {
		replicas := int32(1)
		if dep.Spec.Replicas != nil {
			replicas = *dep.Spec.Replicas
		}

		image := ""
		if len(dep.Spec.Template.Spec.Containers) > 0 {
			image = dep.Spec.Template.Spec.Containers[0].Image
		}

		version := dep.Labels["app.kubernetes.io/version"]

		workflows = append(workflows, WorkflowInfo{
			Name:      dep.Name,
			Namespace: dep.Namespace,
			Version:   version,
			Ready:     dep.Status.ReadyReplicas == replicas,
			Replicas:  replicas,
			Available: dep.Status.AvailableReplicas,
			Image:     image,
			Created:   dep.CreationTimestamp.Time,
		})
	}

	return workflows, nil
}

// GetPodLogs returns a ReadCloser streaming logs from the first running pod matching the workflow name.
// The caller is responsible for closing the reader and managing ctx cancellation for follow mode.
func (c *Client) GetPodLogs(ctx context.Context, namespace, name string, follow bool, tailLines int64) (io.ReadCloser, error) {
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=" + name,
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods found for workflow %s", name)
	}

	// Prefer a Running pod
	podName := pods.Items[0].Name
	for _, p := range pods.Items {
		if p.Status.Phase == corev1.PodRunning {
			podName = p.Name
			break
		}
	}

	logOpts := &corev1.PodLogOptions{
		Follow:    follow,
		TailLines: &tailLines,
	}

	stream, err := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, logOpts).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("streaming logs from pod %s: %w", podName, err)
	}

	return stream, nil
}

// RunWorkflow triggers a deployed workflow by creating a temporary curl pod that POSTs to the workflow service.
// Uses --retry and --retry-connrefused so that if NetworkPolicy ipsets haven't synced the pod's IP
// yet (kube-router race), curl retries automatically with exponential backoff instead of failing.
// Returns the JSON result from stdout. The temp pod is cleaned up on completion.
func (c *Client) RunWorkflow(ctx context.Context, namespace, name string) (string, error) {
	svcURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:8080/run", name, namespace)
	podName := fmt.Sprintf("tntc-run-%s-%d", name, time.Now().UnixMilli())

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "tentacular",
				"tentacular/run-target":        name,
				"tentacular.dev/role":          "trigger",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:  "curl",
					Image: "curlimages/curl:latest",
					Command: []string{
						"curl", "-sf",
						"--retry", "5",
						"--retry-connrefused",
						"--retry-delay", "1",
						"-X", "POST",
						"-H", "Content-Type: application/json",
						"-d", "{}",
						svcURL,
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("32Mi"),
							corev1.ResourceCPU:    resource.MustParse("100m"),
						},
					},
				},
			},
		},
	}

	created, err := c.clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("creating runner pod: %w", err)
	}

	// Clean up the temp pod when done
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = c.clientset.CoreV1().Pods(namespace).Delete(cleanupCtx, podName, metav1.DeleteOptions{})
	}()

	// Watch pod until Succeeded or Failed
	watcher, err := c.clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", podName).String(),
		Watch:         true,
	})
	if err != nil {
		return "", fmt.Errorf("watching runner pod: %w", err)
	}
	defer watcher.Stop()

	// Check if already completed before watching
	if created.Status.Phase == corev1.PodSucceeded || created.Status.Phase == corev1.PodFailed {
		// Already done, skip watch
	} else {
		for event := range watcher.ResultChan() {
			if event.Type == watch.Error {
				return "", fmt.Errorf("watch error")
			}
			p, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}
			if p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
				break
			}
		}
	}

	// Get the final pod status to check phase
	finalPod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting runner pod status: %w", err)
	}

	// Capture stdout logs
	logOpts := &corev1.PodLogOptions{}
	logStream, err := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, logOpts).Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("reading runner pod logs: %w", err)
	}
	defer logStream.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, logStream); err != nil {
		return "", fmt.Errorf("copying runner pod output: %w", err)
	}

	if finalPod.Status.Phase == corev1.PodFailed {
		return "", fmt.Errorf("workflow run failed: %s", buf.String())
	}

	return buf.String(), nil
}

// GetDetailedStatus returns extended deployment information including pod details and events.
func (c *Client) GetDetailedStatus(namespace, name string) (*DetailedStatus, error) {
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

	ds := &DetailedStatus{
		DeploymentStatus: DeploymentStatus{
			Name:      name,
			Namespace: namespace,
			Ready:     dep.Status.ReadyReplicas == replicas,
			Replicas:  replicas,
			Available: dep.Status.AvailableReplicas,
		},
	}

	// Image
	if len(dep.Spec.Template.Spec.Containers) > 0 {
		container := dep.Spec.Template.Spec.Containers[0]
		ds.Image = container.Image

		// Resource limits
		limits := make(map[string]string)
		for k, v := range container.Resources.Limits {
			limits[string(k)] = v.String()
		}
		if len(limits) > 0 {
			ds.ResourceLimits = limits
		}
	}

	// RuntimeClass
	if dep.Spec.Template.Spec.RuntimeClassName != nil {
		ds.RuntimeClass = *dep.Spec.Template.Spec.RuntimeClassName
	}

	// Service endpoint
	svc, err := c.clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		if len(svc.Spec.Ports) > 0 {
			ds.ServiceEndpoint = fmt.Sprintf("%s:%d", svc.Spec.ClusterIP, svc.Spec.Ports[0].Port)
		}
	}

	// Pod statuses
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=" + name,
	})
	if err == nil {
		for _, p := range pods.Items {
			ready := false
			for _, c := range p.Status.Conditions {
				if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
					ready = true
					break
				}
			}
			ds.Pods = append(ds.Pods, PodStatus{
				Name:  p.Name,
				Phase: string(p.Status.Phase),
				Ready: ready,
				IP:    p.Status.PodIP,
			})
		}
	}

	// Recent events
	eventList, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "involvedObject.name=" + name,
	})
	if err == nil {
		for _, e := range eventList.Items {
			ds.Events = append(ds.Events, fmt.Sprintf("%s %s: %s", e.LastTimestamp.Format(time.RFC3339), e.Reason, e.Message))
		}
	}

	return ds, nil
}

// RolloutRestart triggers a rolling restart of a deployment by updating the restartedAt annotation.
func (c *Client) RolloutRestart(namespace, deploymentName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), k8sTimeout)
	defer cancel()

	// Use strategic merge patch to update the pod template annotation
	timestamp := time.Now().Format(time.RFC3339)
	patchData := fmt.Sprintf(`{
		"spec": {
			"template": {
				"metadata": {
					"annotations": {
						"kubectl.kubernetes.io/restartedAt": "%s"
					}
				}
			}
		}
	}`, timestamp)

	_, err := c.clientset.AppsV1().Deployments(namespace).Patch(
		ctx,
		deploymentName,
		types.StrategicMergePatchType,
		[]byte(patchData),
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("patching deployment %s: %w", deploymentName, err)
	}

	return nil
}

// GetNetworkPolicy retrieves a NetworkPolicy resource.
func (c *Client) GetNetworkPolicy(namespace, name string) (*unstructured.Unstructured, error) {
	ctx, cancel := context.WithTimeout(context.Background(), k8sTimeout)
	defer cancel()

	gvr := schema.GroupVersionResource{
		Group:    "networking.k8s.io",
		Version:  "v1",
		Resource: "networkpolicies",
	}

	return c.dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetConfigMap retrieves a ConfigMap resource.
func (c *Client) GetConfigMap(namespace, name string) (*corev1.ConfigMap, error) {
	ctx, cancel := context.WithTimeout(context.Background(), k8sTimeout)
	defer cancel()

	return c.clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetSecret retrieves a Secret resource.
func (c *Client) GetSecret(namespace, name string) (*corev1.Secret, error) {
	ctx, cancel := context.WithTimeout(context.Background(), k8sTimeout)
	defer cancel()

	return c.clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetCronJobs retrieves all CronJobs with a specific label selector.
func (c *Client) GetCronJobs(namespace string, labelSelector string) ([]unstructured.Unstructured, error) {
	ctx, cancel := context.WithTimeout(context.Background(), k8sTimeout)
	defer cancel()

	gvr := schema.GroupVersionResource{
		Group:    "batch",
		Version:  "v1",
		Resource: "cronjobs",
	}

	list, err := c.dynamic.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

// EnsureNamespace creates the given namespace if it does not already exist.
func (c *Client) EnsureNamespace(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), k8sTimeout)
	defer cancel()

	_, err := c.clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return nil // already exists
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("checking namespace %s: %w", name, err)
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "tentacular",
			},
		},
	}
	_, err = c.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating namespace %s: %w", name, err)
	}
	return nil
}
