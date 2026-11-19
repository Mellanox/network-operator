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

package docadriverimages

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"
)

var _ = Describe("DOCA driver pull-secret API access", func() {
	const namespace = "operator-credentials"
	const secretPath = "/api/v1/namespaces/" + namespace + "/secrets/driver-pull-secret"

	BeforeEach(func() {
		stateConfig := &config.FromEnv().State
		previousNamespace := stateConfig.NetworkOperatorResourceNamespace
		stateConfig.NetworkOperatorResourceNamespace = namespace
		DeferCleanup(func() {
			stateConfig.NetworkOperatorResourceNamespace = previousNamespace
		})
	})

	It("uses only named GETs and reloads rotated registry credentials", func() {
		var expectedPassword atomic.Value
		expectedPassword.Store("initial-test-password")
		authenticatedTagRequests := make(chan bool, 2)
		registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, password, ok := r.BasicAuth()
			authenticated := ok && user == "test-user" && password == expectedPassword.Load().(string)
			if !authenticated {
				w.Header().Set("WWW-Authenticate", `Basic realm="test-registry"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if strings.HasSuffix(r.URL.Path, "/tags/list") {
				authenticatedTagRequests <- authenticated
				_, _ = w.Write([]byte(`{"name":"mellanox/doca","tags":["precompiled"]}`))
				return
			}
			_, _ = w.Write([]byte(`{}`))
		}))
		DeferCleanup(registry.Close)
		registryHost := strings.TrimPrefix(registry.URL, "http://")

		var secretResponse atomic.Value
		setSecret := func(password string) {
			credentials, err := json.Marshal(map[string]interface{}{
				"auths": map[string]interface{}{
					registryHost: map[string]string{"username": "test-user", "password": password},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			secret := &corev1.Secret{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
				ObjectMeta: metav1.ObjectMeta{Name: "driver-pull-secret", Namespace: namespace},
				Type:       corev1.SecretTypeDockerConfigJson,
				Data:       map[string][]byte{corev1.DockerConfigJsonKey: credentials},
			}
			response, err := json.Marshal(secret)
			Expect(err).NotTo(HaveOccurred())
			secretResponse.Store(response)
		}
		setSecret("initial-test-password")
		apiRequests := make(chan string, 4)
		kubeClient := pullSecretAPIClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiRequests <- r.Method + " " + r.URL.RequestURI()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(secretResponse.Load().([]byte))
		}))
		p := &provider{
			c: kubeClient,
			docaImageSpec: &mellanoxv1alpha1.ImageSpec{
				Repository: registryHost + "/mellanox", Image: "doca", Version: "test",
				ImagePullSecrets: []string{"driver-pull-secret"},
			},
			ctx: context.Background(),
		}

		p.retrieveTags()
		Expect(p.lastError).NotTo(HaveOccurred())
		Expect(p.tags).To(Equal([]string{"precompiled"}))
		Expect(authenticatedTagRequests).To(Receive(BeTrue()))

		expectedPassword.Store("rotated-test-password")
		setSecret("rotated-test-password")
		p.retrieveTags()
		Expect(p.lastError).NotTo(HaveOccurred())
		Expect(authenticatedTagRequests).To(Receive(BeTrue()))
		Expect(apiRequests).To(HaveLen(2))
		Expect(apiRequests).To(Receive(Equal("GET " + secretPath)))
		Expect(apiRequests).To(Receive(Equal("GET " + secretPath)))
	})

	DescribeTable("Secret read failures",
		func(status int, forbidden bool) {
			apiRequests := make(chan string, 4)
			kubeClient := pullSecretAPIClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				apiRequests <- r.Method + " " + r.URL.RequestURI()
				w.WriteHeader(status)
			}))
			registryCalls := 0
			p := &provider{
				c: kubeClient,
				docaImageSpec: &mellanoxv1alpha1.ImageSpec{
					Repository: "registry.example.com/mellanox", Image: "doca", Version: "test",
					ImagePullSecrets: []string{"driver-pull-secret"},
				},
				ctx: context.Background(),
				tagLister: func(_ name.Repository, _ ...remote.Option) ([]string, error) {
					registryCalls++
					return []string{"precompiled"}, nil
				},
			}

			p.retrieveTags()
			Expect(apiRequests).To(HaveLen(1))
			Expect(apiRequests).To(Receive(Equal("GET " + secretPath)))
			if forbidden {
				Expect(apierrors.IsForbidden(p.lastError)).To(BeTrue())
				Expect(registryCalls).To(BeZero())
			} else {
				Expect(p.lastError).NotTo(HaveOccurred())
				Expect(registryCalls).To(Equal(1))
				Expect(p.tags).To(Equal([]string{"precompiled"}))
			}
		},
		Entry("propagates forbidden reads without querying the registry", http.StatusForbidden, true),
		Entry("continues without an optional missing pull secret", http.StatusNotFound, false),
	)
})

// Match production's uncached client and avoid unrelated API discovery requests.
func pullSecretAPIClient(handler http.Handler) client.Client {
	server := httptest.NewServer(handler)
	DeferCleanup(server.Close)
	scheme := runtime.NewScheme()
	Expect(corev1.AddToScheme(scheme)).To(Succeed())
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	mapper.Add(corev1.SchemeGroupVersion.WithKind("Secret"), meta.RESTScopeNamespace)
	kubeClient, err := client.New(&rest.Config{Host: server.URL}, client.Options{Scheme: scheme, Mapper: mapper})
	Expect(err).NotTo(HaveOccurred())
	return kubeClient
}
