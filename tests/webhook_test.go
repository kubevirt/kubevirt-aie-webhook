package tests

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	alternativeLauncherLabel      = "kubevirt-aie-webhook/alternative-launcher"
	alternativeLauncherAnnotation = "kubevirt.io/alternative-launcher-image"
	pollInterval                  = 2 * time.Second
	pollTimeout                   = 5 * time.Minute
)

func newGuestlessVMI(name, namespace string, labels map[string]string) *kubevirtv1.VirtualMachineInstance {
	terminationGracePeriod := int64(0)
	return &kubevirtv1.VirtualMachineInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: kubevirtv1.VirtualMachineInstanceSpec{
			TerminationGracePeriodSeconds: &terminationGracePeriod,
			Domain: kubevirtv1.DomainSpec{
				Resources: kubevirtv1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
		},
	}
}

func waitForVirtLauncherPod(namespace, vmiName string) *corev1.Pod {
	var pod *corev1.Pod
	Eventually(func(g Gomega) {
		podList := &corev1.PodList{}
		g.Expect(k8sClient.List(context.Background(), podList,
			client.InNamespace(namespace),
			client.MatchingLabels{
				"kubevirt.io":        "virt-launcher",
				"vm.kubevirt.io/name": vmiName,
			},
		)).To(Succeed())
		g.Expect(podList.Items).NotTo(BeEmpty(), "no virt-launcher pod found for VMI %s", vmiName)
		pod = &podList.Items[0]
	}, pollTimeout, pollInterval).Should(Succeed())
	return pod
}

var _ = Describe("Webhook functional tests", func() {
	Context("when a VMI has the alternative-launcher label", func() {
		It("should mutate the virt-launcher pod image", func() {
			vmi := newGuestlessVMI("test-mutated", testNamespace, map[string]string{
				alternativeLauncherLabel: "true",
			})
			Expect(k8sClient.Create(context.Background(), vmi)).To(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Delete(context.Background(), vmi)
			})

			pod := waitForVirtLauncherPod(testNamespace, vmi.Name)
			Expect(pod.Spec.Containers[0].Image).To(Equal(develAltLauncherImage),
				"expected compute container image to be the devel_alt launcher image")
			Expect(pod.Annotations).To(HaveKey(alternativeLauncherAnnotation),
				"expected alternative-launcher-image annotation to be set")
		})
	})

	Context("when a VMI does not have the alternative-launcher label", func() {
		It("should not mutate the virt-launcher pod image", func() {
			vmi := newGuestlessVMI("test-not-mutated", testNamespace, nil)
			Expect(k8sClient.Create(context.Background(), vmi)).To(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Delete(context.Background(), vmi)
			})

			pod := waitForVirtLauncherPod(testNamespace, vmi.Name)
			Expect(pod.Spec.Containers[0].Image).NotTo(ContainSubstring("devel_alt"),
				"expected compute container image to not contain devel_alt")
			Expect(pod.Annotations).NotTo(HaveKey(alternativeLauncherAnnotation),
				"expected alternative-launcher-image annotation to not be set")
		})
	})
})
