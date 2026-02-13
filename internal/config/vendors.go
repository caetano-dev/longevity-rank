package config

import "longevity-ranker/internal/models"

func GetVendors() []models.Vendor {
	return []models.Vendor{
		{
			Name:     "ProHealth",
			URL:      "https://www.prohealth.com/collections/nmn-capsules/products.json",
			Type:     "shopify",
		},
		{
			Name:     "Renue By Science",
			URL:      "https://renuebyscience.com/collections/nmn/products.json",
			Type:     "shopify",
		},
		{
			Name:     "NMN Bio",
			URL:      "https://nmnbio.co.uk/collections/all-products/products.json?currency=USD",
			Type:     "shopify",
		},
		{
			Name:       "Jinfiniti",
			URL:        "https://www.jinfiniti.com/shop/",
			Type:       "html-ldjson",
			Cloudflare: true,
		},
		{
			Name:     "Do Not Age",
			URL:      "https://donotage.org/products/",
			Type:     "magento",
		},
		{
			Name:     "Nutricost",
			URL:      "https://nutricost.com/collections/all-items/products.json",
			Type:     "shopify",
		},
		{
			Name:       "Wonderfeel",
			URL:        "https://www.wonderfeel.com/collections/all/products.json",
			Type:       "shopify",
			Cloudflare: true,
		},
	}
}