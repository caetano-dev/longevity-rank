package scraper

import (
	"fmt"
	"longevity-ranker/internal/models"
)

func FetchProducts(vendor models.Vendor) ([]models.Product, error) {
	switch vendor.Type {
	case "shopify":
		return FetchShopifyProducts(vendor)
	case "html-ldjson":
		return FetchLdJsonProducts(vendor)
	default:
		return nil, fmt.Errorf("unknown vendor scraper type: %s", vendor.Type)
	}
}