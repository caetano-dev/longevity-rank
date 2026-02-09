package config

import "longevity-ranker/internal/models"

func GetVendors() []models.Vendor {
	return []models.Vendor{
		// to add: wonderfeel (cloudflare)
		//         do not age
		{
			Name: "ProHealth",
			URL:  "https://www.prohealth.com/collections/nmn-capsules/products.json",
			Type: "shopify",
		},
		{
			Name: "Renue By Science",
			URL:  "https://renuebyscience.com/collections/nmn/products.json",
			Type: "shopify",
		},
		{
			Name: "NMN Bio",
			URL:  "https://nmnbio.co.uk/collections/all-products/products.json", // TODO: fix product filtering. Some products are not NMN but are still being analyzed,
																				// while others are NMN and are being ignored because there is not enough info on the
																				// dosage, like number of capsules. See https://nmnbio.co.uk/collections/all-products/products/nmn-supplement-500mg-capsules-30-caps?variant=42125023150324
																				// Maybe we could download the image and use OCR to extract the data? If the image URL didn't change,
																				// no need to analyze it again every day.
																				// This website only has 2 NMN supplements. Maybe some hardcoded rules will be enough.
																				// Remeber to include the 1, 3, 6 and 12 bottles options (that don't ahave have a discount)
         	Type: "shopify",
		},
		{
			Name: "Jinfiniti",
			URL:  "https://www.jinfiniti.com/shop/",
			Type: "html-ldjson",
		},
	}
}