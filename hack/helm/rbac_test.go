/*
Copyright 2026 NVIDIA CORPORATION & AFFILIATES

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helm_test

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func TestOperatorRBAC(t *testing.T) {
	helm := os.Getenv("HELM_BIN")
	if helm == "" {
		t.Skip("run make test-helm to test rendered chart RBAC")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	cases := []struct {
		name      string
		release   string
		namespace string
		fullname  string
		values    []string
	}{
		{name: "defaults", release: "network-operator", namespace: "nvidia-network-operator", fullname: "network-operator"},
		{name: "custom release and namespace", release: "fabric", namespace: "operators",
			fullname: "fabric-network-operator"},
		{name: "name override", release: "fabric", namespace: "operators", fullname: "fabric-networking",
			values: []string{"--set", "operator.nameOverride=networking"}},
		{name: "fullname override without hooks or subcharts", release: "fabric", namespace: "operators", fullname: "custom",
			values: []string{"--set", "operator.fullnameOverride=custom,keepNCP=false,upgradeCRDs=false," +
				"nfd.enabled=false,sriovNetworkOperator.enabled=false,maintenanceOperator.enabled=false"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, upgrade := range []bool{false, true} {
				mode := "install"
				if upgrade {
					mode = "upgrade"
				}
				t.Run(mode, func(t *testing.T) {
					args := []string{"template", tc.release, filepath.Join(root, "deployment", "network-operator"),
						"--namespace", tc.namespace, "--kube-version", "1.32.0"}
					args = append(args, tc.values...)
					if upgrade {
						args = append(args, "--is-upgrade")
					}
					output, err := exec.Command(helm, args...).CombinedOutput()
					t.Logf("helm template %s produced %d bytes", mode, len(output))
					require.NoError(t, err, "%s", output)
					checkOperatorRBAC(t, decodeObjects(t, output), tc.fullname, tc.namespace, root)
				})
			}
		})
	}
}

// Decode only the resource kinds under test; CRs and subchart resources do not
// need to be registered in the test scheme.
func decodeObjects(t *testing.T, manifest []byte) map[string]runtime.Object {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, rbacv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	decoder := serializer.NewCodecFactory(scheme).UniversalDeserializer()
	stream := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 4096)
	objects := make(map[string]runtime.Object)
	for {
		var raw runtime.RawExtension
		err := stream.Decode(&raw)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if len(raw.Raw) == 0 {
			continue
		}
		var meta metav1.TypeMeta
		require.NoError(t, yaml.Unmarshal(raw.Raw, &meta))
		kinds := []string{"ClusterRole", "ClusterRoleBinding", "Role", "RoleBinding", "ServiceAccount", "Deployment"}
		if !slices.Contains(kinds, meta.Kind) {
			continue
		}
		obj, _, err := decoder.Decode(raw.Raw, nil, nil)
		require.NoError(t, err)
		metadata, ok := obj.(metav1.Object)
		require.True(t, ok)
		key := meta.Kind + "/" + metadata.GetNamespace() + "/" + metadata.GetName()
		require.NotContains(t, objects, key, "duplicate rendered resource")
		objects[key] = obj
	}
	return objects
}

func checkOperatorRBAC(t *testing.T, objects map[string]runtime.Object, fullname, namespace, root string) {
	t.Helper()
	subjects := []rbacv1.Subject{{Kind: "ServiceAccount", Name: fullname, Namespace: namespace}}
	role, ok := objects["ClusterRole//"+fullname].(*rbacv1.ClusterRole)
	require.True(t, ok, "operator ClusterRole must preserve its existing name")
	binding, ok := objects["ClusterRoleBinding//"+fullname].(*rbacv1.ClusterRoleBinding)
	require.True(t, ok, "operator ClusterRoleBinding must preserve its existing name")
	require.Equal(t, subjects, binding.Subjects,
		"default and unrelated ServiceAccounts must not inherit operator permissions")
	require.Equal(t, rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: fullname}, binding.RoleRef)
	deployment, ok := objects["Deployment/"+namespace+"/"+fullname].(*appsv1.Deployment)
	require.True(t, ok)
	require.Equal(t, fullname, deployment.Spec.Template.Spec.ServiceAccountName)
	require.Contains(t, objects, "ServiceAccount/"+namespace+"/"+fullname)

	for _, obj := range objects {
		if otherBinding, ok := obj.(*rbacv1.ClusterRoleBinding); ok && otherBinding.RoleRef == binding.RoleRef {
			require.Equal(t, subjects, otherBinding.Subjects, "all bindings of the operator role must be exclusive")
		}
		if clusterRole, ok := obj.(*rbacv1.ClusterRole); ok {
			for _, rule := range clusterRole.Rules {
				if slices.Contains(rule.APIGroups, "") || slices.Contains(rule.APIGroups, "*") {
					require.NotContains(t, rule.Resources, "secrets", "cluster-wide Secret grant in %s", clusterRole.Name)
					require.NotContains(t, rule.Resources, "*", "wildcard core grant in %s", clusterRole.Name)
				}
			}
		}
	}
	secretRole, ok := objects["Role/"+namespace+"/"+fullname+"-secret-reader"].(*rbacv1.Role)
	require.True(t, ok, "Secret access must be scoped to the operator namespace")
	expectedSecretRules := []rbacv1.PolicyRule{{
		APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"},
	}}
	require.Equal(t, expectedSecretRules, secretRole.Rules)
	secretBinding, ok := objects["RoleBinding/"+namespace+"/"+fullname+"-secret-reader"].(*rbacv1.RoleBinding)
	require.True(t, ok)
	require.Equal(t, subjects, secretBinding.Subjects)
	require.Equal(t, rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: fullname + "-secret-reader"},
		secretBinding.RoleRef)

	checkGeneratedRBAC(t, root, role, secretRole)
}

func checkGeneratedRBAC(t *testing.T, root string, role *rbacv1.ClusterRole, secretRole *rbacv1.Role) {
	t.Helper()
	generated, err := os.ReadFile(filepath.Join(root, "config", "rbac", "role.yaml"))
	require.NoError(t, err)
	generatedObjects := decodeObjects(t, generated)
	generatedRole, ok := generatedObjects["ClusterRole//manager-role"].(*rbacv1.ClusterRole)
	require.True(t, ok)
	require.Equal(t, generatedRole.Rules, role.Rules, "Helm must retain the generated operator permissions")
	generatedSecretRole, ok := generatedObjects["Role/system/manager-secret-reader"].(*rbacv1.Role)
	require.True(t, ok)
	require.Equal(t, generatedSecretRole.Rules, secretRole.Rules, "Helm and Kustomize Secret permissions must match")

	bindings, err := os.ReadFile(filepath.Join(root, "config", "rbac", "role_binding.yaml"))
	require.NoError(t, err)
	generatedBindings := decodeObjects(t, bindings)
	expectedSubjects := []rbacv1.Subject{{Kind: "ServiceAccount", Name: "controller-manager", Namespace: "system"}}
	managerBinding, ok := generatedBindings["ClusterRoleBinding//manager-rolebinding"].(*rbacv1.ClusterRoleBinding)
	require.True(t, ok)
	require.Equal(t, expectedSubjects, managerBinding.Subjects)
	require.Equal(t, rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: generatedRole.Name},
		managerBinding.RoleRef)
	readerBinding, ok := generatedBindings["RoleBinding/system/manager-secret-reader"].(*rbacv1.RoleBinding)
	require.True(t, ok)
	require.Equal(t, expectedSubjects, readerBinding.Subjects)
	require.Equal(t, rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: generatedSecretRole.Name},
		readerBinding.RoleRef)
}
