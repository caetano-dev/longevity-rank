package scraper

import (
	"fmt"
	"longevity-ranker/internal/models"
)

// FetchFunc is the signature that all scraper backends implement.
type FetchFunc func(models.Vendor) ([]models.Product, error)

// registry maps vendor type strings to their scraper implementation.
var registry = map[string]FetchFunc{
	"shopify":    FetchShopifyProducts,
	"html-ldjson": FetchLdJsonProducts,
	"magento":    FetchMagentoProducts,
}

// FetchProducts dispatches to the correct scraper based on vendor.Type.
func FetchProducts(vendor models.Vendor) ([]models.Product, error) {
	fn, ok := registry[vendor.Type]
	if !ok {
		return nil, fmt.Errorf("unknown vendor scraper type: %s", vendor.Type)
	}
	return fn(vendor)
}