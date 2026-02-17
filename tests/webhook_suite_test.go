package tests

import (
	"context"
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	k8sClient          client.Client
	testNamespace      string
	develAltLauncherImage string
)

func TestFunctional(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Functional Test Suite")
}

var _ = BeforeSuite(func() {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kubevirtv1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	cfg, err := kubeConfig.ClientConfig()
	Expect(err).NotTo(HaveOccurred(), "failed to build kubeconfig")

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred(), "failed to create client")

	// Create test namespace
	testNamespace = "kubevirt-aie-webhook-functest"
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
		},
	}
	err = k8sClient.Create(context.Background(), ns)
	Expect(err).NotTo(HaveOccurred(), "failed to create test namespace")

	// Discover virt-launcher devel_alt image from virt-api deployment
	virtAPI := &appsv1.Deployment{}
	err = k8sClient.Get(context.Background(), client.ObjectKey{
		Namespace: "kubevirt",
		Name:      "virt-api",
	}, virtAPI)
	Expect(err).NotTo(HaveOccurred(), "failed to get virt-api deployment")

	virtAPIImage := virtAPI.Spec.Template.Spec.Containers[0].Image
	// Image is like registry:5000/kubevirt/virt-api:devel
	// We need registry:5000/kubevirt/virt-launcher:devel_alt
	lastSlash := strings.LastIndex(virtAPIImage, "/")
	Expect(lastSlash).To(BeNumerically(">", 0), "unexpected virt-api image format: %s", virtAPIImage)
	imagePrefix := virtAPIImage[:lastSlash]
	develAltLauncherImage = fmt.Sprintf("%s/virt-launcher:devel_alt", imagePrefix)
})

var _ = AfterSuite(func() {
	if k8sClient == nil {
		return
	}
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
		},
	}
	err := k8sClient.Delete(context.Background(), ns)
	Expect(err).NotTo(HaveOccurred(), "failed to delete test namespace")
})
