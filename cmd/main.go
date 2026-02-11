package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"path/filepath"

	"longevity-ranker/internal/config"
	"longevity-ranker/internal/models"
	"longevity-ranker/internal/parser"
	"longevity-ranker/internal/rules"
	"longevity-ranker/internal/scraper"
	"longevity-ranker/internal/storage"
)

func main() {
	// Define command line flag
	refresh := flag.Bool("refresh", false, "Scrape websites to update local data")
	flag.Parse()

	// Initialize storage folder
	if err := storage.EnsureDataDir(); err != nil {
		panic(err)
	}
	
	rulesPath := filepath.Join("data", "vendor_rules.json")
		if err := rules.LoadRules(rulesPath); err != nil {
			fmt.Printf("⚠️ Warning: Could not load rules (%v). Running without filters.\n", err)
		} 

	vendors := config.GetVendors()
	var allProducts []struct {
		VendorName string
		Product    models.Product
	}

	// --- PHASE 1: DATA GATHERING ---
	for _, v := range vendors {
		var products []models.Product
		var err error

		// Logic: Scrape if -refresh flag is set, OR if local file is missing
		shouldScrape := *refresh
		if !shouldScrape {
			_, err := os.Stat(storage.GetFilename(v.Name))
			if os.IsNotExist(err) {
				shouldScrape = true
			}
		}

		if shouldScrape {
			// A. SCRAPE & SAVE
			products, err = scraper.FetchProducts(v)
			if err != nil {
				fmt.Printf("❌ Error scraping %s: %v\n", v.Name, err)
				continue
			}
			// Save to JSON for debugging/future runs
			if err := storage.SaveProducts(v.Name, products); err != nil {
				fmt.Printf("⚠️ Error saving data for %s: %v\n", v.Name, err)
			}
			fmt.Printf("✅ Saved %d products for %s\n", len(products), v.Name)
		} else {
			// B. LOAD FROM LOCAL
			products, err = storage.LoadProducts(v.Name)
			if err != nil {
				fmt.Printf("❌ Error loading %s: %v\n", v.Name, err)
				continue
			}
			// fmt.Printf("📂 Loaded %d products for %s from cache\n", len(products), v.Name)
		}

		// Collect for analysis
		for _, p := range products {
			// --- NEW: NORMALIZATION LAYER ---
			// Apply vendor-specific rules to filter bad products or fix missing data.
			// We pass a pointer (&p) so the rule can modify the product (Enrichment).
			keep := rules.ApplyRules(v.Name, &p)
			if !keep {
				continue // Skip non-NMN products or blacklisted items
			}

			allProducts = append(allProducts, struct {
				VendorName string
				Product    models.Product
			}{v.Name, p})
		}
	}

	// --- PHASE 2: ANALYSIS ---
	var report []models.Analysis

	for _, item := range allProducts {
		analysis := parser.AnalyzeProduct(item.VendorName, item.Product)
		if analysis != nil {
			report = append(report, *analysis)
		}
	}

	// --- PHASE 3: REPORTING ---
	// Sort by ROI (Cheapest first)
	sort.Slice(report, func(i, j int) bool {
		return report[i].CostPerGram < report[j].CostPerGram
	})

	printTable(report)
}

func printTable(data []models.Analysis) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\nRANK\tVENDOR\tPRODUCT (Truncated)\tTYPE\tPRICE\tGRAMS\t$/GRAM")
	fmt.Fprintln(w, "----\t------\t-------------------\t-----\t-----\t-----\t------")

	for i, row := range data {
		// Truncate long titles
		shortName := row.Name
		if len(shortName) > 35 {
			shortName = shortName[:32] + "..."
		}

		reset := "\033[0m"
		color := reset
		if row.CostPerGram < 0.6 {
			color = "\033[31m" // Red (Too cheap/Suspicious)
		} else if row.CostPerGram < 1.5 {
			color = "\033[32m" // Green (Great Deal)
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t$%.2f\t%.1fg\t%s$%.2f%s\n",
			i+1, row.Vendor, shortName, row.Type, row.Price, row.TotalGrams, color, row.CostPerGram, reset)
	}
	w.Flush()
}