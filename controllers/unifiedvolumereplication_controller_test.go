/*
Copyright 2024.

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

package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg/translation"
)

var _ = Describe("UnifiedVolumeReplicationController", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When reconciling a UnifiedVolumeReplication", func() {
		var (
			reconciler     *UnifiedVolumeReplicationReconciler
			ctx            context.Context
			uvr            *replicationv1alpha1.UnifiedVolumeReplication
			namespacedName types.NamespacedName
		)

		BeforeEach(func() {
			ctx = context.Background()

			// Create a fake client
			s := scheme.Scheme
			Expect(replicationv1alpha1.AddToScheme(s)).To(Succeed())

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithStatusSubresource(&replicationv1alpha1.UnifiedVolumeReplication{}).
				Build()

			// Create reconciler
			reconciler = &UnifiedVolumeReplicationReconciler{
				Client:   fakeClient,
				Log:      ctrl.Log.WithName("controllers").WithName("UnifiedVolumeReplication"),
				Scheme:   s,
				Recorder: record.NewFakeRecorder(100),
			}

			// Create test resource
			namespacedName = types.NamespacedName{
				Name:      "test-replication",
				Namespace: "default",
			}

			uvr = &replicationv1alpha1.UnifiedVolumeReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      namespacedName.Name,
					Namespace: namespacedName.Namespace,
				},
				Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
					ReplicationState: replicationv1alpha1.ReplicationStateReplica,
					ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
					VolumeMapping: replicationv1alpha1.VolumeMapping{
						Source: replicationv1alpha1.VolumeSource{
							PvcName:   "source-pvc",
							Namespace: "default",
						},
						Destination: replicationv1alpha1.VolumeDestination{
							VolumeHandle: "dest-volume",
							Namespace:    "default",
						},
					},
					SourceEndpoint: replicationv1alpha1.Endpoint{
						Cluster:      "source-cluster",
						Region:       "us-east-1",
						StorageClass: "fast-ssd",
					},
					DestinationEndpoint: replicationv1alpha1.Endpoint{
						Cluster:      "dest-cluster",
						Region:       "us-west-1",
						StorageClass: "fast-ssd",
					},
					Schedule: replicationv1alpha1.Schedule{
						Mode: replicationv1alpha1.ScheduleModeContinuous,
						Rpo:  "15m",
						Rto:  "5m",
					},
					Extensions: &replicationv1alpha1.Extensions{
						Trident: &replicationv1alpha1.TridentExtensions{
							Actions: []replicationv1alpha1.TridentAction{},
						},
					},
				},
			}
		})

		It("should add finalizer on first reconcile", func() {
			Expect(reconciler.Create(ctx, uvr)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))

			// Fetch updated resource
			updatedUVR := &replicationv1alpha1.UnifiedVolumeReplication{}
			Expect(reconciler.Get(ctx, namespacedName, updatedUVR)).To(Succeed())
			Expect(updatedUVR.Finalizers).To(ContainElement(unifiedReplicationFinalizer))
		})

		It("should update status conditions", func() {
			Expect(reconciler.Create(ctx, uvr)).To(Succeed())

			// First reconcile adds finalizer
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Second reconcile processes the resource
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})
			// May error if adapter not available, but should update status

			// Fetch updated resource
			updatedUVR := &replicationv1alpha1.UnifiedVolumeReplication{}
			Expect(reconciler.Get(ctx, namespacedName, updatedUVR)).To(Succeed())

			// Should have conditions
			Eventually(func() bool {
				_ = reconciler.Get(ctx, namespacedName, updatedUVR)
				return len(updatedUVR.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())
		})

		It("should handle resource deletion with finalizer cleanup", func() {
			// Add finalizer manually for this test
			uvr.Finalizers = []string{unifiedReplicationFinalizer}
			Expect(reconciler.Create(ctx, uvr)).To(Succeed())

			// Mark for deletion
			Expect(reconciler.Delete(ctx, uvr)).To(Succeed())

			// Reconcile to handle deletion
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Resource should be deleted
			deletedUVR := &replicationv1alpha1.UnifiedVolumeReplication{}
			err = reconciler.Get(ctx, namespacedName, deletedUVR)
			Expect(client.IgnoreNotFound(err)).To(Succeed())
		})

		It("should handle not found gracefully", func() {
			// Don't create the resource
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically("==", 0))
		})

		It("should update observed generation", func() {
			Expect(reconciler.Create(ctx, uvr)).To(Succeed())

			// Reconcile twice
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})

			// Check observed generation
			updatedUVR := &replicationv1alpha1.UnifiedVolumeReplication{}
			Eventually(func() int64 {
				_ = reconciler.Get(ctx, namespacedName, updatedUVR)
				return updatedUVR.Status.ObservedGeneration
			}, timeout, interval).Should(Equal(uvr.Generation))
		})
	})

	Context("Condition management", func() {
		var (
			reconciler *UnifiedVolumeReplicationReconciler
			uvr        *replicationv1alpha1.UnifiedVolumeReplication
		)

		BeforeEach(func() {
			s := scheme.Scheme
			Expect(replicationv1alpha1.AddToScheme(s)).To(Succeed())

			reconciler = &UnifiedVolumeReplicationReconciler{
				Log:    ctrl.Log.WithName("test"),
				Scheme: s,
			}

			uvr = &replicationv1alpha1.UnifiedVolumeReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Namespace:  "default",
					Generation: 1,
				},
				Status: replicationv1alpha1.UnifiedVolumeReplicationStatus{
					Conditions: []metav1.Condition{},
				},
			}
		})

		It("should add new condition", func() {
			condition := metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				Reason:             "TestReason",
				Message:            "Test message",
				ObservedGeneration: 1,
			}

			reconciler.updateCondition(uvr, condition)
			Expect(uvr.Status.Conditions).To(HaveLen(1))
			Expect(uvr.Status.Conditions[0].Type).To(Equal("Ready"))
			Expect(uvr.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
		})

		It("should update existing condition", func() {
			// Add initial condition
			initialCondition := metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "Initial",
				Message:            "Initial message",
				ObservedGeneration: 1,
			}
			reconciler.updateCondition(uvr, initialCondition)

			// Update condition
			updatedCondition := metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				Reason:             "Updated",
				Message:            "Updated message",
				ObservedGeneration: 1,
			}
			reconciler.updateCondition(uvr, updatedCondition)

			Expect(uvr.Status.Conditions).To(HaveLen(1))
			Expect(uvr.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			Expect(uvr.Status.Conditions[0].Reason).To(Equal("Updated"))
		})

		It("should get condition by type", func() {
			condition := metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionTrue,
			}
			reconciler.updateCondition(uvr, condition)

			found := reconciler.getCondition(uvr, "Ready")
			Expect(found).NotTo(BeNil())
			Expect(found.Type).To(Equal("Ready"))

			notFound := reconciler.getCondition(uvr, "NonExistent")
			Expect(notFound).To(BeNil())
		})
	})

	// Operation determination tests removed (behavior now handled by EnsureReplication)

	Context("Adapter selection", func() {
		var (
			reconciler *UnifiedVolumeReplicationReconciler
			ctx        context.Context
		)

		BeforeEach(func() {
			ctx = context.Background()
			s := scheme.Scheme
			Expect(replicationv1alpha1.AddToScheme(s)).To(Succeed())

			fakeClient := fake.NewClientBuilder().WithScheme(s).Build()

			reconciler = &UnifiedVolumeReplicationReconciler{
				Client: fakeClient,
				Log:    ctrl.Log.WithName("test"),
				Scheme: s,
			}
		})

		It("should select Trident adapter when Trident extensions present", func() {
			uvr := &replicationv1alpha1.UnifiedVolumeReplication{
				Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
					Extensions: &replicationv1alpha1.Extensions{
						Trident: &replicationv1alpha1.TridentExtensions{},
					},
				},
			}

			adapter, err := reconciler.getAdapter(ctx, uvr, reconciler.Log)
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter).NotTo(BeNil())
			Expect(adapter.GetBackendType()).To(Equal(translation.BackendTrident))
		})

		It("should select PowerStore adapter when PowerStore extensions present", func() {
			uvr := &replicationv1alpha1.UnifiedVolumeReplication{
				Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
					Extensions: &replicationv1alpha1.Extensions{
						Powerstore: &replicationv1alpha1.PowerStoreExtensions{},
					},
				},
			}

			adapter, err := reconciler.getAdapter(ctx, uvr, reconciler.Log)
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter).NotTo(BeNil())
			Expect(adapter.GetBackendType()).To(Equal(translation.BackendPowerStore))
		})

		It("should error when no backend extensions present", func() {
			uvr := &replicationv1alpha1.UnifiedVolumeReplication{
				Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
					Extensions: &replicationv1alpha1.Extensions{},
				},
			}

			adapter, err := reconciler.getAdapter(ctx, uvr, reconciler.Log)
			Expect(err).To(HaveOccurred())
			Expect(adapter).To(BeNil())
		})
	})
})
