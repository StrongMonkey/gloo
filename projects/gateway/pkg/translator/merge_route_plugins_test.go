package translator

import (
	"time"

	"github.com/gogo/protobuf/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "github.com/solo-io/gloo/projects/gloo/pkg/api/v1"
	"github.com/solo-io/gloo/projects/gloo/pkg/api/v1/enterprise/options/ratelimit"
	"github.com/solo-io/gloo/projects/gloo/pkg/api/v1/options/retries"
)

var _ = Describe("MergeRoutePlugins", func() {
	It("merges top-level route plugins fields", func() {
		dst := &v1.Options{
			PrefixRewrite: &types.StringValue{Value: "preserve-me"},
			Retries: &retries.RetryPolicy{
				RetryOn:    "5XX",
				NumRetries: 0, // should not overwrite this field
			},
		}
		d := time.Minute
		src := &v1.Options{
			Timeout: &d,
			Retries: &retries.RetryPolicy{
				RetryOn:    "5XX",
				NumRetries: 3, // do not overwrite 0 value above
			},
			RatelimitBasic: &ratelimit.IngressRateLimit{
				AuthorizedLimits: &ratelimit.RateLimit{
					Unit:            1,
					RequestsPerUnit: 2,
				},
			},
		}
		expected := &v1.Options{
			PrefixRewrite: &types.StringValue{Value: "preserve-me"},
			Timeout:       &d,
			Retries: &retries.RetryPolicy{
				RetryOn:    "5XX",
				NumRetries: 0,
			},
			RatelimitBasic: &ratelimit.IngressRateLimit{
				AuthorizedLimits: &ratelimit.RateLimit{
					Unit:            1,
					RequestsPerUnit: 2,
				},
			},
		}

		actual, err := mergeRoutePlugins(dst, src)
		Expect(err).NotTo(HaveOccurred())
		Expect(actual).To(Equal(expected))
	})
})
