package di

import (
	"context"
	"log"
	"time"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/ports"
)

func rehydrateBloomFilter(ctx context.Context, repo ports.URLRepository, bloom ports.BloomFilter, batchSize int) {
	start := time.Now()
	total := 0

	err := repo.IterateAliases(ctx, batchSize, func(aliases []string) error {
		if err := bloom.AddMany(ctx, aliases); err != nil {
			return err
		}
		total += len(aliases)
		return nil
	})
	if err != nil {
		log.Printf("bloom rehydrate aborted after %d aliases in %s: %v",
			total, time.Since(start), err)
		return
	}
	log.Printf("bloom rehydrate completed: %d aliases in %s", total, time.Since(start))
}
