package services_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lihai1/stat-tree-server/internal/services"
)

var _ = Describe("LotteryManager", func() {
	Context("with a nil repository (no DB)", func() {
		It("should return an empty archive without error", func() {
			m := services.NewLotteryManager(nil)
			arch, err := m.Archive(context.Background(), time.Time{}, time.Time{})
			Expect(err).ToNot(HaveOccurred())
			Expect(arch).ToNot(BeNil())
			Expect(arch.Size()).To(Equal(0))
		})

		It("should apply the default start date when from is zero", func() {
			m := services.NewLotteryManager(nil)
			arch, err := m.Archive(context.Background(), time.Time{}, time.Time{})
			Expect(err).ToNot(HaveOccurred())
			// The default from date is 2004-02-12.
			Expect(arch.From).To(Equal(time.Date(2004, 2, 12, 0, 0, 0, 0, time.UTC)))
		})

		It("should cache the archive and reuse it", func() {
			m := services.NewLotteryManager(nil)
			_, _ = m.Archive(context.Background(), time.Time{}, time.Time{})
			Expect(m.CacheSize()).To(Equal(1))
			_, _ = m.Archive(context.Background(), time.Time{}, time.Time{})
			Expect(m.CacheSize()).To(Equal(1))
		})

		It("should cache distinct windows separately", func() {
			m := services.NewLotteryManager(nil)
			from1 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
			from2 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
			_, _ = m.Archive(context.Background(), from1, time.Time{})
			_, _ = m.Archive(context.Background(), from2, time.Time{})
			Expect(m.CacheSize()).To(Equal(2))
		})
	})

	Context("InvalidateRange", func() {
		It("should remove only overlapping windows", func() {
			m := services.NewLotteryManager(nil)
			w1 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
			w2 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
			_, _ = m.Archive(context.Background(), w1, time.Time{})
			_, _ = m.Archive(context.Background(), w2, time.Time{})
			Expect(m.CacheSize()).To(Equal(2))

			// Invalidate only the window overlapping 2020 draws.
			m.InvalidateRange(
				time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2020, 6, 15, 0, 0, 0, 0, time.UTC),
			)
			Expect(m.CacheSize()).To(Equal(1))
		})

		It("should retain non-overlapping windows", func() {
			m := services.NewLotteryManager(nil)
			w1 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
			w1To := time.Date(2020, 12, 31, 0, 0, 0, 0, time.UTC)
			_, _ = m.Archive(context.Background(), w1, w1To)
			m.InvalidateRange(
				time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2030, 12, 31, 0, 0, 0, 0, time.UTC),
			)
			Expect(m.CacheSize()).To(Equal(1))
		})

		It("should clear everything on InvalidateAll", func() {
			m := services.NewLotteryManager(nil)
			_, _ = m.Archive(context.Background(), time.Time{}, time.Time{})
			_, _ = m.Archive(context.Background(),
				time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), time.Time{})
			m.InvalidateAll()
			Expect(m.CacheSize()).To(Equal(0))
		})
	})
})
