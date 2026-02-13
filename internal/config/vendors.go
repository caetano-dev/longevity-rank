package config

import "longevity-ranker/internal/models"

func GetVendors() []models.Vendor {
	return []models.Vendor{
		{
			Name:     "ProHealth",
			URL:      "https://www.prohealth.com/collections/nmn-capsules/products.json",
			Type:     "shopify",
			Currency: "USD",
		},
		{
			Name:     "Renue By Science",
			URL:      "https://renuebyscience.com/collections/nmn/products.json",
			Type:     "shopify",
			Currency: "USD",
		},
		{
			Name:     "NMN Bio",
			URL:      "https://nmnbio.co.uk/collections/all-products/products.json?currency=USD",
			Type:     "shopify",
			Currency: "USD",
		},
		{
			Name:       "Jinfiniti",
			URL:        "https://www.jinfiniti.com/shop/",
			Type:       "html-ldjson",
			Currency:   "USD",
			Cloudflare: true,
		},
		{
			Name:     "Do Not Age",
			URL:      "https://donotage.org/products/",
			Type:     "magento",
			Currency: "USD",
		},
		{
			Name:     "Nutricost",
			URL:      "https://nutricost.com/collections/all-items/products.json",
			Type:     "shopify",
			Currency: "USD",
		},
		{
			Name:       "Wonderfeel",
			URL:        "https://www.wonderfeel.com/collections/all/products.json",
			Type:       "shopify",
			Currency:   "USD",
			Cloudflare: true,
		},
	}
}