package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CheckResult represents a single preflight check result.
type CheckResult struct {
	Name        string `json:"name"`
	Passed      bool   `json:"passed"`
	Warning     string `json:"warning,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

// CheckResultsJSON serializes a slice of CheckResult as a JSON array.
func CheckResultsJSON(results []CheckResult) string {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Sprintf(`[{"error": "failed to marshal results: %s"}]`, err)
	}
	return string(data)
}

// PreflightCheck validates cluster readiness for tentacular deployments.
// secretNames is an optional list of K8s Secret names to verify in the target namespace.
func (c *Client) PreflightCheck(namespace string, autoFix bool, secretNames []string) ([]CheckResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var results []CheckResult

	// 1. K8s API reachable
	_, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		results = append(results, CheckResult{
			Name:        "K8s API reachable",
			Passed:      false,
			Remediation: fmt.Sprintf("Cannot reach K8s API: %s", err),
		})
		// Early termination: all remaining checks depend on API connectivity
		return results, nil
	}
	results = append(results, CheckResult{Name: "K8s API reachable", Passed: true})

	// 2. gVisor RuntimeClass exists (warning if missing, not failure)
	_, err = c.clientset.NodeV1().RuntimeClasses().Get(ctx, "gvisor", metav1.GetOptions{})
	if err != nil {
		results = append(results, CheckResult{
			Name:    "gVisor RuntimeClass",
			Passed:  true,
			Warning: "gVisor RuntimeClass not found. Deployments will run without gVisor sandboxing. Install with: sudo bash deploy/gvisor/install.sh && kubectl apply -f deploy/gvisor/runtimeclass.yaml",
		})
	} else {
		results = append(results, CheckResult{Name: "gVisor RuntimeClass", Passed: true})
	}

	// 3. Namespace exists (with auto-creation via --fix)
	_, err = c.clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		if autoFix {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: namespace},
			}
			_, createErr := c.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
			if createErr != nil {
				results = append(results, CheckResult{
					Name:        fmt.Sprintf("Namespace '%s'", namespace),
					Passed:      false,
					Remediation: fmt.Sprintf("Failed to auto-create namespace: %s", createErr),
				})
			} else {
				results = append(results, CheckResult{
					Name:   fmt.Sprintf("Namespace '%s' (auto-created)", namespace),
					Passed: true,
				})
			}
		} else {
			results = append(results, CheckResult{
				Name:        fmt.Sprintf("Namespace '%s'", namespace),
				Passed:      false,
				Remediation: fmt.Sprintf("Run: kubectl create namespace %s (or use --fix)", namespace),
			})
		}
	} else {
		results = append(results, CheckResult{Name: fmt.Sprintf("Namespace '%s'", namespace), Passed: true})
	}

	// 4. RBAC permissions via SelfSubjectAccessReview
	rbacResult := c.checkRBACPermissions(ctx, namespace)
	results = append(results, rbacResult)

	// 5. Secret references check
	if len(secretNames) > 0 {
		secretResult := c.checkSecretReferences(ctx, namespace, secretNames)
		results = append(results, secretResult)
	} else {
		results = append(results, CheckResult{
			Name:        "Secret references",
			Passed:      true,
			Remediation: "No workflow spec found; secret reference check skipped",
		})
	}

	return results, nil
}

// checkRBACPermissions uses SelfSubjectAccessReview to verify the current identity
// can create, update, and delete Deployments, Services, ConfigMaps, and Secrets.
func (c *Client) checkRBACPermissions(ctx context.Context, namespace string) CheckResult {
	type permCheck struct {
		group    string
		resource string
		verb     string
	}

	checks := []permCheck{
		{group: "apps", resource: "deployments", verb: "create"},
		{group: "apps", resource: "deployments", verb: "update"},
		{group: "apps", resource: "deployments", verb: "delete"},
		{group: "", resource: "services", verb: "create"},
		{group: "", resource: "services", verb: "update"},
		{group: "", resource: "services", verb: "delete"},
		{group: "", resource: "configmaps", verb: "create"},
		{group: "", resource: "configmaps", verb: "update"},
		{group: "", resource: "configmaps", verb: "delete"},
		{group: "", resource: "secrets", verb: "create"},
		{group: "", resource: "secrets", verb: "update"},
		{group: "", resource: "secrets", verb: "delete"},
		{group: "batch", resource: "cronjobs", verb: "create"},
		{group: "batch", resource: "cronjobs", verb: "update"},
		{group: "batch", resource: "cronjobs", verb: "delete"},
		{group: "batch", resource: "cronjobs", verb: "list"},
		{group: "batch", resource: "jobs", verb: "list"},
	}

	var missing []string
	for _, check := range checks {
		sar := &authorizationv1.SelfSubjectAccessReview{
			Spec: authorizationv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Namespace: namespace,
					Verb:      check.verb,
					Group:     check.group,
					Resource:  check.resource,
				},
			},
		}

		result, err := c.clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
		if err != nil {
			return CheckResult{
				Name:        "RBAC permissions",
				Passed:      false,
				Remediation: fmt.Sprintf("Failed to check RBAC permissions: %s. Verify your kubeconfig context.", err),
			}
		}

		if !result.Status.Allowed {
			missing = append(missing, fmt.Sprintf("%s %s", check.verb, check.resource))
		}
	}

	if len(missing) > 0 {
		return CheckResult{
			Name:        "RBAC permissions",
			Passed:      false,
			Remediation: fmt.Sprintf("Missing permissions in namespace %s: %s", namespace, strings.Join(missing, ", ")),
		}
	}

	return CheckResult{Name: "RBAC permissions", Passed: true}
}

// CheckModuleProxy checks whether the esm.sh module proxy Deployment is installed and
// ready in the given namespace. Returns a non-blocking CheckResult so that
// `tntc cluster check` stays informational for clusters that don't use jsr/npm deps:
//   - Not installed → Passed=true with a Warning hint
//   - Installed but not ready → Passed=false with a Remediation hint
//   - Installed and ready → Passed=true
func (c *Client) CheckModuleProxy(namespace string) CheckResult {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dep, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, "esm-sh", metav1.GetOptions{})
	if err != nil {
		// Not installed: informational warning only — no jsr/npm workflows = not needed.
		return CheckResult{
			Name:    "Module proxy (esm.sh)",
			Passed:  true,
			Warning: "not installed in " + namespace + " \u2014 run `tntc cluster install` to enable jsr/npm dep support",
		}
	}

	replicas := int32(1)
	if dep.Spec.Replicas != nil {
		replicas = *dep.Spec.Replicas
	}
	if dep.Status.ReadyReplicas < replicas {
		return CheckResult{
			Name:    "Module proxy (esm.sh)",
			Passed:  false,
			Warning: fmt.Sprintf("not ready (%d/%d replicas ready)", dep.Status.ReadyReplicas, replicas),
			Remediation: fmt.Sprintf(
				"check pod logs: kubectl logs -n %s -l app.kubernetes.io/name=esm-sh",
				namespace,
			),
		}
	}

	return CheckResult{Name: "Module proxy (esm.sh)", Passed: true}
}

// checkSecretReferences verifies that each named secret exists in the target namespace.
func (c *Client) checkSecretReferences(ctx context.Context, namespace string, secretNames []string) CheckResult {
	var missing []string
	for _, name := range secretNames {
		_, err := c.clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return CheckResult{
			Name:        "Secret references",
			Passed:      false,
			Remediation: fmt.Sprintf("Missing secrets in namespace %s: %s", namespace, strings.Join(missing, ", ")),
		}
	}

	return CheckResult{Name: "Secret references", Passed: true}
}
