package tlssettings_test

import (
	"crypto/tls"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"kubevirt.io/kubevirt-aie-webhook/pkg/tlssettings"
)

func TestTLSSettings(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TLS Settings Suite")
}

var _ = Describe("TLSSettings", func() {

	Context("happy cases", func() {
		It("should do nothing if no tls curveIDs is configured", func() {
			var orig []func(*tls.Config)
			result, err := tlssettings.AppendCurveIds(GinkgoLogr, orig, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(0))
		})

		It("should add known CurveIDs", func() {
			var orig []func(*tls.Config)

			result, err := tlssettings.AppendCurveIds(GinkgoLogr, orig, "23,24,25,29,4588,4587,4589")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
			cfg := &tls.Config{}
			result[0](cfg)
			Expect(cfg.CurvePreferences).To(ConsistOf(
				tls.CurveP256,
				tls.CurveP384,
				tls.CurveP521,
				tls.X25519,
				tls.X25519MLKEM768,
				tls.SecP256r1MLKEM768,
				tls.SecP384r1MLKEM1024,
			))
		})
	})

	Context("error cases", func() {
		It("should skip ID if it is not numeric", func() {
			var orig []func(*tls.Config)

			result, err := tlssettings.AppendCurveIds(GinkgoLogr, orig, "23,24,somethingelse,25")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
			cfg := &tls.Config{}
			result[0](cfg)
			Expect(cfg.CurvePreferences).To(ConsistOf(
				tls.CurveP256,
				tls.CurveP384,
				tls.CurveP521,
			))
		})

		It("should skip ID if it is unknown", func() {
			var orig []func(*tls.Config)

			result, err := tlssettings.AppendCurveIds(GinkgoLogr, orig, "23,24,1000,25")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
			cfg := &tls.Config{}
			result[0](cfg)
			Expect(cfg.CurvePreferences).To(ConsistOf(
				tls.CurveP256,
				tls.CurveP384,
				tls.CurveP521,
			))
		})

		It("should return error if no valid curve ID is found", func() {
			var orig []func(*tls.Config)

			Expect(tlssettings.AppendCurveIds(GinkgoLogr, orig, "1,2,c,3")).Error().To(MatchError(ContainSubstring("no valid curveIds found")))
		})
	})
})
