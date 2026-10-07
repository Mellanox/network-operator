/*
  2026 NVIDIA CORPORATION & AFFILIATES

  Licensed under the Apache License, Version 2.0 (the License);
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an AS IS BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
*/

package state

import (
	"context"
	"maps"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/utils"
)

// staleTestTaint reproduces the lab scenario: any taint the policy does not tolerate drops the
// node out of pool discovery. A custom key is used rather than not-ready so the test does not
// depend on which taints are tolerated by default.
const staleTestTaint = "repro.nvidia.com/untolerated"

// assignUIDOnCreate makes the fake client stamp a UID like the API server does, so a test can
// tell a preserved object from one that was deleted and recreated under the same name.
var assignUIDOnCreate = interceptor.Funcs{
	Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
		if obj.GetUID() == "" {
			obj.SetUID(uuid.NewUUID())
		}
		return c.Create(ctx, obj, opts...)
	},
}

var _ = Describe("MOFED stale DaemonSet cleanup", func() {
	var (
		ctx       context.Context
		k8sClient client.Client
		ofedState *stateOFED
		cr        *v1alpha1.NicClusterPolicy
	)

	// sync reconciles the OFED state against exactly the given set of cluster nodes.
	sync := func(nodes ...*v1.Node) {
		catalog := NewInfoCatalog()
		catalog.Add(InfoTypeClusterType, &dummyProvider{})
		catalog.Add(InfoTypeNodeInfo, nodeinfo.NewProvider(nodes))
		catalog.Add(InfoTypeDocaDriverImage, &dummyOfedImageProvider{tagExists: true})
		_, err := ofedState.Sync(ctx, cr, catalog)
		Expect(err).NotTo(HaveOccurred())
	}

	daemonSets := func() []appsv1.DaemonSet {
		list := &appsv1.DaemonSetList{}
		Expect(k8sClient.List(ctx, list, client.MatchingLabels{consts.StateLabel: stateOFEDName})).To(Succeed())
		return list.Items
	}

	// theDaemonSet asserts a single OFED DaemonSet exists and returns it.
	theDaemonSet := func() *appsv1.DaemonSet {
		items := daemonSets()
		Expect(items).To(HaveLen(1))
		return &items[0]
	}

	daemonSetFor := func(kernel string) *appsv1.DaemonSet {
		pool := nodeinfo.PoolName(osName, osVer, kernel)
		items := daemonSets()
		for i := range items {
			if name, ok := ofedPoolNameFromNodeSelector(items[i].Spec.Template.Spec.NodeSelector); ok && name == pool {
				return &items[i]
			}
		}
		Fail("no OFED DaemonSet rendered for pool " + pool)
		return nil
	}

	// backdateStaleSince rewinds the stale marker to simulate time passing between reconciles.
	backdateStaleSince := func(ds *appsv1.DaemonSet, age time.Duration) {
		patched := ds.DeepCopy()
		annotations := patched.GetAnnotations()
		annotations[consts.StaleSinceAnnotation] = time.Now().UTC().Add(-age).Format(time.RFC3339)
		patched.SetAnnotations(annotations)
		Expect(k8sClient.Patch(ctx, patched, client.MergeFrom(ds))).To(Succeed())
	}

	BeforeEach(func() {
		ctx = context.Background()

		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(v1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		k8sClient = fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(assignUIDOnCreate).Build()

		files, err := utils.GetFilesWithSuffix("../../manifests/state-ofed-driver", render.ManifestFileSuffix...)
		Expect(err).NotTo(HaveOccurred())
		ofedState = &stateOFED{stateSkel: stateSkel{
			name:        stateOFEDName,
			description: stateOFEDDescription,
			client:      k8sClient,
			renderer:    render.NewRenderer(files),
		}}

		cr = &v1alpha1.NicClusterPolicy{}
		cr.Name = consts.NicClusterPolicyResourceName
		cr.Spec.OFEDDriver = &v1alpha1.OFEDDriverSpec{
			ImageSpec: v1alpha1.ImageSpec{
				Image:      "doca-driver",
				Repository: "nvcr.io/nvidia/mellanox",
				Version:    "24.10-0.7.0.0-0",
			},
		}
	})

	Context("a node pool with no eligible nodes", func() {
		var (
			node        *v1.Node
			taintedNode *v1.Node
			originalUID types.UID
		)

		BeforeEach(func() {
			node = getNode("node1", kernelFull1)
			taintedNode = node.DeepCopy()
			taintedNode.Spec.Taints = []v1.Taint{
				{Key: staleTestTaint, Value: "true", Effect: v1.TaintEffectNoSchedule},
			}

			sync(node)
			originalUID = theDaemonSet().GetUID()
			Expect(originalUID).NotTo(BeEmpty())
		})

		It("keeps the DaemonSet of a one-node pool whose only node is excluded", func() {
			sync(taintedNode)

			ds := theDaemonSet()
			Expect(ds.GetUID()).To(Equal(originalUID), "the DaemonSet must not be replaced")
			Expect(ds.GetAnnotations()).To(HaveKey(consts.StaleSinceAnnotation))
			Expect(ofedState.RequeueAfter()).To(And(BeNumerically(">", 0),
				BeNumerically("<=", staleOFEDGracePeriod)))
		})

		It("keeps the objects the retained DaemonSet needs to restart its pod", func() {
			sync(taintedNode)

			sa := &v1.ServiceAccount{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "ofed-driver",
				Namespace: config.FromEnv().State.NetworkOperatorResourceNamespace,
			}, sa)).To(Succeed())
		})

		It("cancels the deferred cleanup when the pool becomes eligible again", func() {
			sync(taintedNode)
			Expect(theDaemonSet().GetAnnotations()).To(HaveKey(consts.StaleSinceAnnotation))

			sync(node)

			ds := theDaemonSet()
			Expect(ds.GetUID()).To(Equal(originalUID), "the DaemonSet must not be recreated on recovery")
			Expect(ds.GetAnnotations()).NotTo(HaveKey(consts.StaleSinceAnnotation))
			Expect(ofedState.RequeueAfter()).To(BeZero())
		})

		It("preserves the first-observed time across reconciles and operator restarts", func() {
			sync(taintedNode)
			stamped := theDaemonSet().GetAnnotations()[consts.StaleSinceAnnotation]

			backdateStaleSince(theDaemonSet(), 10*time.Minute)
			backdated := theDaemonSet().GetAnnotations()[consts.StaleSinceAnnotation]
			Expect(backdated).NotTo(Equal(stamped))

			// A fresh state object is what the operator starts with after a restart or a
			// leader-election handover: the only record of the deadline is on the DaemonSet.
			files, err := utils.GetFilesWithSuffix("../../manifests/state-ofed-driver", render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			ofedState = &stateOFED{stateSkel: stateSkel{
				name:        stateOFEDName,
				description: stateOFEDDescription,
				client:      k8sClient,
				renderer:    render.NewRenderer(files),
			}}

			sync(taintedNode)
			sync(taintedNode)

			Expect(theDaemonSet().GetAnnotations()[consts.StaleSinceAnnotation]).To(Equal(backdated),
				"repeated reconciles must not restart the grace period")
			Expect(ofedState.RequeueAfter()).To(BeNumerically("<=", staleOFEDGracePeriod-10*time.Minute))
		})

		It("reaps the DaemonSet once the pool has been absent for the whole grace period", func() {
			sync(taintedNode)
			backdateStaleSince(theDaemonSet(), staleOFEDGracePeriod+time.Minute)

			sync(taintedNode)

			Expect(daemonSets()).To(BeEmpty())
			Expect(ofedState.RequeueAfter()).To(BeZero())
		})

		It("starts a fresh grace period after a recovery followed by another loss", func() {
			sync(taintedNode)
			backdateStaleSince(theDaemonSet(), staleOFEDGracePeriod-time.Minute)

			sync(node)
			sync(taintedNode)

			ds := theDaemonSet()
			Expect(ds.GetUID()).To(Equal(originalUID))
			since, err := time.Parse(time.RFC3339, ds.GetAnnotations()[consts.StaleSinceAnnotation])
			Expect(err).NotTo(HaveOccurred())
			Expect(time.Since(since)).To(BeNumerically("<", time.Minute))
		})

		It("deletes the DaemonSet immediately when ofedDriver is removed from the policy", func() {
			sync(taintedNode)
			Expect(daemonSets()).To(HaveLen(1))

			cr.Spec.OFEDDriver = nil
			sync(taintedNode)

			Expect(daemonSets()).To(BeEmpty())
		})
	})

	It("keeps the DaemonSet when every node of a multi-node pool is excluded at once", func() {
		nodes := []*v1.Node{getNode("node1", kernelFull1), getNode("node2", kernelFull1), getNode("node3", kernelFull1)}
		sync(nodes...)
		originalUID := theDaemonSet().GetUID()

		tainted := make([]*v1.Node, 0, len(nodes))
		for _, node := range nodes {
			taintedNode := node.DeepCopy()
			taintedNode.Spec.Taints = []v1.Taint{
				{Key: staleTestTaint, Value: "true", Effect: v1.TaintEffectNoSchedule},
			}
			tainted = append(tainted, taintedNode)
		}
		sync(tainted...)

		Expect(theDaemonSet().GetUID()).To(Equal(originalUID))
	})

	It("keeps one pool's DaemonSet without touching a pool that still has nodes", func() {
		poolA := getNode("node-pool-a", kernelFull1)
		poolB := getNode("node-pool-b", kernelFull2)
		sync(poolA, poolB)
		Expect(daemonSets()).To(HaveLen(2))
		uidA := daemonSetFor(kernelFull1).GetUID()
		uidB := daemonSetFor(kernelFull2).GetUID()

		taintedA := poolA.DeepCopy()
		taintedA.Spec.Taints = []v1.Taint{{Key: staleTestTaint, Value: "true", Effect: v1.TaintEffectNoSchedule}}
		sync(taintedA, poolB)

		Expect(daemonSets()).To(HaveLen(2))
		Expect(daemonSetFor(kernelFull1).GetUID()).To(Equal(uidA))
		Expect(daemonSetFor(kernelFull1).GetAnnotations()).To(HaveKey(consts.StaleSinceAnnotation))
		Expect(daemonSetFor(kernelFull2).GetUID()).To(Equal(uidB))
		Expect(daemonSetFor(kernelFull2).GetAnnotations()).NotTo(HaveKey(consts.StaleSinceAnnotation))
		Expect(ofedState.RequeueAfter()).To(BeNumerically(">", 0),
			"the surviving pool keeps the policy otherwise event-driven, so the deadline needs a requeue")
	})

	Context("a driver version change", func() {
		const newVersion = "25.01-0.1.0.0-0"

		// driverImage returns the image of the single rendered OFED DaemonSet. The DaemonSet is
		// named after its pool and not after the driver version, so a version change rewrites
		// this field rather than producing a second DaemonSet.
		driverImage := func() string {
			return theDaemonSet().Spec.Template.Spec.Containers[0].Image
		}

		It("is applied in place while the pool has nodes", func() {
			node := getNode("node1", kernelFull1)
			sync(node)
			Expect(driverImage()).To(ContainSubstring(cr.Spec.OFEDDriver.Version))

			cr.Spec.OFEDDriver.Version = newVersion
			sync(node)

			Expect(driverImage()).To(ContainSubstring(newVersion))
			Expect(theDaemonSet().GetAnnotations()).NotTo(HaveKey(consts.StaleSinceAnnotation))
			Expect(ofedState.RequeueAfter()).To(BeZero())
		})

		It("waits for the pool to come back when it has no nodes, and does not strand it", func() {
			node := getNode("node1", kernelFull1)
			oldVersion := cr.Spec.OFEDDriver.Version
			sync(node)

			taintedNode := node.DeepCopy()
			taintedNode.Spec.Taints = []v1.Taint{
				{Key: staleTestTaint, Value: "true", Effect: v1.TaintEffectNoSchedule},
			}
			cr.Spec.OFEDDriver.Version = newVersion
			sync(taintedNode)

			// A pool with no visible nodes yields no rendered DaemonSet, so there is nothing to
			// upgrade to. Deleting the old one instead would take the driver away from nodes
			// that are still running it, which is the failure this whole mechanism exists to
			// prevent, so the upgrade waits rather than the workload being torn down.
			Expect(driverImage()).To(ContainSubstring(oldVersion))
			Expect(theDaemonSet().GetAnnotations()).To(HaveKey(consts.StaleSinceAnnotation))

			sync(node)

			Expect(driverImage()).To(ContainSubstring(newVersion))
			Expect(theDaemonSet().GetAnnotations()).NotTo(HaveKey(consts.StaleSinceAnnotation))
		})

		It("is abandoned with the pool once the grace period expires", func() {
			node := getNode("node1", kernelFull1)
			sync(node)

			taintedNode := node.DeepCopy()
			taintedNode.Spec.Taints = []v1.Taint{
				{Key: staleTestTaint, Value: "true", Effect: v1.TaintEffectNoSchedule},
			}
			cr.Spec.OFEDDriver.Version = newVersion
			sync(taintedNode)
			backdateStaleSince(theDaemonSet(), staleOFEDGracePeriod+time.Minute)

			sync(taintedNode)

			Expect(daemonSets()).To(BeEmpty())
		})
	})

	Context("a policy retargeted to different nodes", func() {
		var policy *v1alpha1.NicNodePolicy

		// syncPolicy reconciles the NicNodePolicy against the nodes it currently selects, which
		// is what the controller hands the state: setupOFEDCatalog lists nodes by the policy's
		// nodeSelector, so a node the policy stopped selecting is simply absent from the catalog.
		syncPolicy := func(nodes ...*v1.Node) {
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			catalog.Add(InfoTypeNodeInfo, nodeinfo.NewProvider(nodes))
			catalog.Add(InfoTypeDocaDriverImage, &dummyOfedImageProvider{tagExists: true})
			_, err := ofedState.Sync(ctx, policy, catalog)
			Expect(err).NotTo(HaveOccurred())
		}

		nodeWithRole := func(name, kernel, role string) *v1.Node {
			node := getNode(name, kernel)
			node.Labels["role"] = role
			return node
		}

		taint := func(node *v1.Node) *v1.Node {
			tainted := node.DeepCopy()
			tainted.Spec.Taints = []v1.Taint{
				{Key: staleTestTaint, Value: "true", Effect: v1.TaintEffectNoSchedule},
			}
			return tainted
		}

		BeforeEach(func() {
			policy = &v1alpha1.NicNodePolicy{}
			policy.Name = "pool-a"
			policy.Spec.NodeSelector = map[string]string{"role": "a"}
			policy.Spec.OFEDDriver = &v1alpha1.OFEDDriverSpec{
				ImageSpec: v1alpha1.ImageSpec{
					Image:      "doca-driver",
					Repository: "nvcr.io/nvidia/mellanox",
					Version:    "24.10-0.7.0.0-0",
				},
			}
		})

		It("renders the policy nodeSelector into the DaemonSet", func() {
			// The retarget check reads a DaemonSet's targeting back off the object itself, so it
			// rests on the manifest rendering the policy's nodeSelector verbatim.
			syncPolicy(nodeWithRole("node1", kernelFull1, "a"))

			Expect(theDaemonSet().Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("role", "a"))
		})

		It("deletes the DaemonSet of a pool the policy no longer selects", func() {
			syncPolicy(nodeWithRole("node1", kernelFull1, "a"))
			Expect(daemonSets()).To(HaveLen(1))

			policy.Spec.NodeSelector = map[string]string{"role": "b"}
			syncPolicy(nodeWithRole("node2", kernelFull2, "b"))

			// Retargeting is an unambiguous intentional change, so the old pool's DaemonSet must
			// not be held for the grace period: its pod would keep the node's one-driver slot and
			// block any policy that has taken the node over.
			Expect(daemonSets()).To(HaveLen(1))
			Expect(daemonSetFor(kernelFull2).GetAnnotations()).NotTo(HaveKey(consts.StaleSinceAnnotation))
			Expect(ofedState.RequeueAfter()).To(BeZero())
		})

		It("abandons a deferral already in progress once the policy is retargeted", func() {
			node1 := nodeWithRole("node1", kernelFull1, "a")
			syncPolicy(node1)
			syncPolicy(taint(node1))
			Expect(theDaemonSet().GetAnnotations()).To(HaveKey(consts.StaleSinceAnnotation))

			policy.Spec.NodeSelector = map[string]string{"role": "b"}
			syncPolicy(nodeWithRole("node2", kernelFull2, "b"))

			Expect(daemonSets()).To(HaveLen(1))
			Expect(daemonSetFor(kernelFull2)).NotTo(BeNil())
		})

		It("still defers when the policy selects the same nodes and the pool empties", func() {
			node1 := nodeWithRole("node1", kernelFull1, "a")
			syncPolicy(node1)
			originalUID := theDaemonSet().GetUID()

			syncPolicy(taint(node1))

			ds := theDaemonSet()
			Expect(ds.GetUID()).To(Equal(originalUID))
			Expect(ds.GetAnnotations()).To(HaveKey(consts.StaleSinceAnnotation))
			Expect(ofedState.RequeueAfter()).To(BeNumerically(">", 0))
		})

		It("still defers when the policy selects on a label the manifest also renders", func() {
			// Selecting on an NFD label is an ordinary thing for a policy to do, and such an
			// entry has to be read back as the policy's own. Treating the manifest's labels as
			// exclusively its own would read this policy's targeting as empty, call an unchanged
			// policy retargeted, and delete the DaemonSet the first time its pool emptied.
			policy.Spec.NodeSelector = map[string]string{nodeinfo.NodeLabelOSTreeVersion: rhcosOsTree}
			node1 := getNode("node1", kernelFull1)
			node1.Labels[nodeinfo.NodeLabelOSTreeVersion] = rhcosOsTree
			syncPolicy(node1)
			Expect(theDaemonSet().Spec.Template.Spec.NodeSelector).To(
				HaveKeyWithValue(nodeinfo.NodeLabelOSTreeVersion, rhcosOsTree))

			syncPolicy(taint(node1))

			Expect(theDaemonSet().GetAnnotations()).To(HaveKey(consts.StaleSinceAnnotation))
			Expect(ofedState.RequeueAfter()).To(BeNumerically(">", 0))
		})
	})

	Context("deciding whether a rendered DaemonSet was retargeted", func() {
		rendered := func(policySelector map[string]string) map[string]string {
			selector := map[string]string{
				nodeinfo.NodeLabelMlnxNIC:       "true",
				nodeinfo.NodeLabelOSName:        osName,
				nodeinfo.NodeLabelOSVer:         osVer,
				nodeinfo.NodeLabelKernelVerFull: kernelFull1,
			}
			maps.Copy(selector, policySelector)
			return selector
		}

		It("says no while the policy asks for what was rendered", func() {
			Expect(ofedSelectorRetargeted(rendered(map[string]string{"role": "a"}),
				map[string]string{"role": "a"})).To(BeFalse())
		})

		It("says yes once the policy asks for a different value", func() {
			Expect(ofedSelectorRetargeted(rendered(map[string]string{"role": "a"}),
				map[string]string{"role": "b"})).To(BeTrue())
		})

		It("says yes once the policy adds a selector the DaemonSet was not rendered for", func() {
			Expect(ofedSelectorRetargeted(rendered(nil), map[string]string{"role": "a"})).To(BeTrue())
		})

		It("says no for a policy selecting on a label the manifest renders too", func() {
			Expect(ofedSelectorRetargeted(rendered(nil),
				map[string]string{nodeinfo.NodeLabelOSVer: osVer})).To(BeFalse())
		})

		It("says no for the absent nodeSelector of a NicClusterPolicy", func() {
			// A NicClusterPolicy has no nodeSelector at all, so its DaemonSets must never look
			// retargeted and the deferral has to stay available to them.
			Expect(ofedSelectorRetargeted(rendered(nil), cr.GetNodeSelector())).To(BeFalse())
		})

		It("says no when the policy drops one of several selectors", func() {
			// Widening keeps every node that matched before, so nothing was taken away on
			// purpose and the pool check alone decides.
			Expect(ofedSelectorRetargeted(rendered(map[string]string{"role": "a", "zone": "x"}),
				map[string]string{"role": "a"})).To(BeFalse())
		})
	})

	It("tolerates the not-ready and unreachable NoSchedule taints when discovering pools", func() {
		node := getNode("node1", kernelFull1)
		node.Spec.Taints = []v1.Taint{
			{Key: v1.TaintNodeNotReady, Effect: v1.TaintEffectNoSchedule},
			{Key: v1.TaintNodeUnreachable, Effect: v1.TaintEffectNoSchedule},
		}

		pools := ofedNodePools(nodeinfo.NewProvider([]*v1.Node{node}), cr)

		Expect(pools).To(HaveLen(1))
		Expect(pools[0].Name).To(Equal(nodeinfo.PoolName(osName, osVer, kernelFull1)))
	})

	It("derives the pool of a rendered DaemonSet from its own nodeSelector", func() {
		node := getNode("node1", kernelFull1)
		provider := nodeinfo.NewProvider([]*v1.Node{node})
		pools := ofedNodePools(provider, cr)
		Expect(pools).To(HaveLen(1))

		sync(node)

		derived, ok := ofedPoolNameFromNodeSelector(theDaemonSet().Spec.Template.Spec.NodeSelector)
		Expect(ok).To(BeTrue())
		Expect(derived).To(Equal(pools[0].Name))
	})

	Context("the deferral of a single DaemonSet", func() {
		var (
			ds        *appsv1.DaemonSet
			livePools map[string]struct{}
			noPools   map[string]struct{}
		)

		BeforeEach(func() {
			sync(getNode("node1", kernelFull1))
			ds = theDaemonSet()
			livePools = map[string]struct{}{nodeinfo.PoolName(osName, osVer, kernelFull1): {}}
			noPools = map[string]struct{}{}
		})

		It("lets a DaemonSet go while its own pool still has eligible nodes", func() {
			remaining, err := ofedState.deferralRemaining(ctx, cr, ds, livePools, time.Now().UTC())

			Expect(err).NotTo(HaveOccurred())
			Expect(remaining).To(BeZero())
			Expect(theDaemonSet().GetAnnotations()).NotTo(HaveKey(consts.StaleSinceAnnotation))
		})

		It("lets a DaemonSet go when no pool can be derived from its nodeSelector", func() {
			ds.Spec.Template.Spec.NodeSelector = map[string]string{nodeinfo.NodeLabelMlnxNIC: "true"}

			remaining, err := ofedState.deferralRemaining(ctx, cr, ds, noPools, time.Now().UTC())

			Expect(err).NotTo(HaveOccurred())
			Expect(remaining).To(BeZero())
			Expect(theDaemonSet().GetAnnotations()).NotTo(HaveKey(consts.StaleSinceAnnotation))
		})

		It("marks the DaemonSet and defers for the whole grace period on first observation", func() {
			now := time.Now().UTC()

			remaining, err := ofedState.deferralRemaining(ctx, cr, ds, noPools, now)

			Expect(err).NotTo(HaveOccurred())
			Expect(remaining).To(Equal(staleOFEDGracePeriod))
			Expect(theDaemonSet().GetAnnotations()).To(HaveKeyWithValue(
				consts.StaleSinceAnnotation, now.Format(time.RFC3339)))
		})

		It("counts down from the first observation instead of restarting it", func() {
			_, err := ofedState.deferralRemaining(ctx, cr, ds, noPools, time.Now().UTC())
			Expect(err).NotTo(HaveOccurred())
			backdateStaleSince(theDaemonSet(), 5*time.Minute)

			remaining, err := ofedState.deferralRemaining(ctx, cr, theDaemonSet(), noPools, time.Now().UTC())

			Expect(err).NotTo(HaveOccurred())
			Expect(remaining).To(BeNumerically("~", staleOFEDGracePeriod-5*time.Minute, time.Minute))
		})

		It("lets a DaemonSet go once the whole grace period has elapsed", func() {
			_, err := ofedState.deferralRemaining(ctx, cr, ds, noPools, time.Now().UTC())
			Expect(err).NotTo(HaveOccurred())
			backdateStaleSince(theDaemonSet(), staleOFEDGracePeriod+time.Minute)

			remaining, err := ofedState.deferralRemaining(ctx, cr, theDaemonSet(), noPools, time.Now().UTC())

			Expect(err).NotTo(HaveOccurred())
			Expect(remaining).To(BeZero())
		})
	})

	Context("the retention bookkeeping of one reconcile", func() {
		object := func(kind, namespace, name string) *unstructured.Unstructured {
			obj := &unstructured.Unstructured{}
			obj.SetKind(kind)
			obj.SetNamespace(namespace)
			obj.SetName(name)
			return obj
		}

		It("indexes the rendered DaemonSets by namespace and name", func() {
			desired := desiredDaemonSetKeys([]*unstructured.Unstructured{
				object("DaemonSet", "nvidia-network-operator", "mofed-ds"),
				object("DaemonSet", "elsewhere", "mofed-ds"),
				object("ServiceAccount", "nvidia-network-operator", "ofed-driver"),
			})

			Expect(desired).To(HaveLen(2))
			Expect(desired).To(HaveKey(types.NamespacedName{Namespace: "nvidia-network-operator", Name: "mofed-ds"}))
			Expect(desired).To(HaveKey(types.NamespacedName{Namespace: "elsewhere", Name: "mofed-ds"}))
		})

		It("keeps the soonest deadline across the protected DaemonSets", func() {
			retention := &staleOFEDRetention{protected: map[types.NamespacedName]struct{}{}}

			retention.protect(types.NamespacedName{Name: "late"}, 15*time.Minute)
			retention.protect(types.NamespacedName{Name: "soon"}, 2*time.Minute)
			retention.protect(types.NamespacedName{Name: "later"}, 30*time.Minute)

			Expect(retention.protected).To(HaveLen(3))
			Expect(retention.nextDeadline).To(Equal(2 * time.Minute))
		})

		It("retains the shared objects only while some DaemonSet is protected", func() {
			retention := &staleOFEDRetention{protected: map[types.NamespacedName]struct{}{}}
			serviceAccount := object("ServiceAccount", "nvidia-network-operator", "ofed-driver")
			daemonSet := object("DaemonSet", "nvidia-network-operator", "mofed-ds")
			Expect(retention.retain(serviceAccount)).To(BeFalse())
			Expect(retention.retain(daemonSet)).To(BeFalse())

			retention.protect(objectKey(daemonSet), time.Minute)

			Expect(retention.retain(daemonSet)).To(BeTrue())
			Expect(retention.retain(object("DaemonSet", "nvidia-network-operator", "other-ds"))).To(BeFalse())
			Expect(retention.retain(serviceAccount)).To(BeTrue())
		})
	})

	It("leaves a DaemonSet without a pool in its nodeSelector to the generic cleanup", func() {
		node := getNode("node1", kernelFull1)
		sync(node)

		ds := theDaemonSet()
		patched := ds.DeepCopy()
		patched.Spec.Template.Spec.NodeSelector = map[string]string{nodeinfo.NodeLabelMlnxNIC: "true"}
		Expect(k8sClient.Update(ctx, patched)).To(Succeed())

		taintedNode := node.DeepCopy()
		taintedNode.Spec.Taints = []v1.Taint{{Key: staleTestTaint, Value: "true", Effect: v1.TaintEffectNoSchedule}}
		sync(taintedNode)

		Expect(daemonSets()).To(BeEmpty())
	})
})
