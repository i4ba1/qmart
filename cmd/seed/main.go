// Command seed loads demo catalog data and per-store inventory so the API can
// be exercised immediately.
package main

import (
	"context"
	"fmt"
	"log"

	"qmart/internal/config"
	"qmart/internal/model"
	"qmart/internal/store"
)

const seedStore = "store-bali-01"

func main() {
	cfg := config.Load()
	st, err := store.Open(cfg.DSN)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()

	products := []model.Product{
		{SKU: "GROC-MILK-1L", Name: "Fresh Milk 1L", Category: "dairy", PriceCents: 2490, Currency: "USD", IsActive: true},
		{SKU: "GROC-EGG-12", Name: "Free-Range Eggs (12)", Category: "dairy", PriceCents: 3990, Currency: "USD", IsActive: true},
		{SKU: "GROC-BREAD", Name: "Sourdough Loaf", Category: "bakery", PriceCents: 5200, Currency: "USD", IsActive: true},
		{SKU: "SNACK-CHOC", Name: "Dark Chocolate Bar", Category: "snacks", PriceCents: 3100, Currency: "USD", IsActive: true},
		{SKU: "BEV-COLD-6", Name: "Cold Brew (6-pack)", Category: "beverages", PriceCents: 8900, Currency: "USD", IsActive: true},
	}

	stock := map[string]int64{
		"GROC-MILK-1L": 120,
		"GROC-EGG-12":  80,
		"GROC-BREAD":   40,
		"SNACK-CHOC":   200,
		"BEV-COLD-6":   60,
	}

	for _, p := range products {
		id, err := st.CreateProduct(ctx, &p)
		if err != nil {
			log.Printf("skip %s: %v", p.SKU, err)
			continue
		}
		if err := st.AdjustInventory(ctx, id, seedStore, stock[p.SKU], 0); err != nil {
			log.Printf("inventory %s: %v", p.SKU, err)
		}
		fmt.Printf("seeded %s (id=%d) qty=%d\n", p.SKU, id, stock[p.SKU])
	}
	fmt.Printf("done. store=%s\n", seedStore)
}
