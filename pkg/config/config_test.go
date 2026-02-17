package config_test

import (
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"kubevirt.io/kubevirt-aie-webhook/pkg/config"
)

var _ = Describe("ConfigStore", func() {
	var store *config.ConfigStore

	BeforeEach(func() {
		store = config.NewConfigStore()
	})

	Describe("Get", func() {
		It("should return nil before any update", func() {
			Expect(store.Get()).To(BeNil())
		})
	})

	Describe("Update", func() {
		It("should parse valid YAML with device names", func() {
			data := []byte(`
rules:
- name: "test-rule"
  image: "registry.example.com/launcher:v1"
  selector:
    deviceNames:
    - "nvidia.com/A100"
`)
			Expect(store.Update(data)).To(Succeed())

			cfg := store.Get()
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.Rules).To(HaveLen(1))
			Expect(cfg.Rules[0].Name).To(Equal("test-rule"))
			Expect(cfg.Rules[0].Image).To(Equal("registry.example.com/launcher:v1"))
			Expect(cfg.Rules[0].Selector.DeviceNames).To(ConsistOf("nvidia.com/A100"))
		})

		It("should parse valid YAML with VM labels", func() {
			data := []byte(`
rules:
- name: "label-rule"
  image: "registry.example.com/launcher:v2"
  selector:
    vmLabels:
      matchLabels:
        aie.kubevirt.io/launcher: "true"
`)
			Expect(store.Update(data)).To(Succeed())

			cfg := store.Get()
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.Rules[0].Selector.VMLabels).ToNot(BeNil())
			Expect(cfg.Rules[0].Selector.VMLabels.MatchLabels).To(HaveKeyWithValue("aie.kubevirt.io/launcher", "true"))
		})

		It("should handle an empty rules list", func() {
			Expect(store.Update([]byte("rules: []"))).To(Succeed())

			cfg := store.Get()
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.Rules).To(BeEmpty())
		})

		It("should return an error for malformed YAML", func() {
			Expect(store.Update([]byte("not: [valid: yaml"))).ToNot(Succeed())
		})

		It("should replace the previous config on subsequent updates", func() {
			first := []byte(`
rules:
- name: "first"
  image: "first:v1"
  selector:
    deviceNames: ["a"]
`)
			second := []byte(`
rules:
- name: "second"
  image: "second:v2"
  selector:
    deviceNames: ["b"]
`)
			Expect(store.Update(first)).To(Succeed())
			Expect(store.Get().Rules[0].Name).To(Equal("first"))

			Expect(store.Update(second)).To(Succeed())
			Expect(store.Get().Rules[0].Name).To(Equal("second"))
		})
	})

	Describe("concurrent access", func() {
		It("should be safe under concurrent reads and writes", func() {
			data := []byte(`
rules:
- name: "concurrent"
  image: "registry.example.com/launcher:v1"
  selector:
    deviceNames:
    - "nvidia.com/A100"
`)
			var wg sync.WaitGroup
			for range 100 {
				wg.Add(2)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					Expect(store.Update(data)).To(Succeed())
				}()
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					_ = store.Get()
				}()
			}
			wg.Wait()
		})
	})
})
