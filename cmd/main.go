package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"longevity-ranker/internal/config"
	"longevity-ranker/internal/models"
	"longevity-ranker/internal/parser"
	"longevity-ranker/internal/rules"
	"longevity-ranker/internal/scraper"
	"longevity-ranker/internal/storage"
)

func main() {
	refresh := flag.Bool("refresh", false, "Scrape websites to update local data")
	audit := flag.Bool("audit", false, "Detect products that need manual overrides in vendor_rules.json")
	supplements := flag.String("supplements", "nmn,nad,tmg,trimethylglycine,resveratrol,creatine", "Comma-separated list of supplement keywords to track")
	flag.Parse()

	// Apply supplement filter to the analyzer's gatekeeper
	if *supplements != "" {
		parts := strings.Split(*supplements, ",")
		var cleaned []string
		for _, s := range parts {
			s = strings.TrimSpace(strings.ToLower(s))
			if s != "" {
				cleaned = append(cleaned, s)
			}
		}
		if len(cleaned) > 0 {
			parser.AllowedSupplements = cleaned
		}
	}

	if err := storage.EnsureDataDir(); err != nil {
		panic(err)
	}

	rulesPath := filepath.Join("data", "vendor_rules.json")
	if err := rules.LoadRules(rulesPath); err != nil {
		fmt.Printf("⚠️ Warning: Could not load rules (%v). Running without filters.\n", err)
	} else {
		fmt.Println("✅ Loaded vendor rules from JSON")
	}

	vendors := config.GetVendors()
	var allProducts []struct {
		VendorName string
		Product    models.Product
	}

	for _, v := range vendors {
		var products []models.Product
		var err error

		shouldScrape := *refresh
		if !shouldScrape {
			_, err := os.Stat(storage.GetFilename(v.Name))
			if os.IsNotExist(err) {
				shouldScrape = true
			}
		}

		// Cloudflare-blocked vendors cannot be scraped automatically.
		// They rely on manually-maintained JSON in the data/ directory.
		if shouldScrape && v.Cloudflare {
			fmt.Printf("🛡️  Skipping %s (Cloudflare-protected). Using local JSON if available.\n", v.Name)
			shouldScrape = false
		}

		if shouldScrape {
			products, err = scraper.FetchProducts(v)
			if err != nil {
				fmt.Printf("❌ Error scraping %s: %v\n", v.Name, err)
				continue
			}
			if err := storage.SaveProducts(v.Name, products); err != nil {
				fmt.Printf("⚠️ Error saving data for %s: %v\n", v.Name, err)
			}
			fmt.Printf("✅ Saved %d products for %s\n", len(products), v.Name)
		} else {
			products, err = storage.LoadProducts(v.Name)
			if err != nil {
				fmt.Printf("❌ Error loading %s: %v\n", v.Name, err)
				continue
			}
		}

		for _, p := range products {
			keep := rules.ApplyRules(v.Name, &p)
			if !keep {
				continue
			}

			allProducts = append(allProducts, struct {
				VendorName string
				Product    models.Product
			}{v.Name, p})
		}
	}

	var report []models.Analysis
	var auditResults []parser.AuditResult

	for _, item := range allProducts {
		analyses := parser.AnalyzeProduct(item.VendorName, item.Product)
		if analyses != nil {
			report = append(report, analyses...)
		}

		if *audit {
			gap := parser.AuditProduct(item.VendorName, item.Product)
			if gap != nil {
				auditResults = append(auditResults, *gap)
			}
		}
	}

	// SORT BY EFFECTIVE COST (TRUE VALUE)
	sort.Slice(report, func(i, j int) bool {
		return report[i].EffectiveCost < report[j].EffectiveCost
	})

	if err := storage.SaveReport(report); err != nil {
		fmt.Printf("⚠️ Error saving analysis report: %v\n", err)
	} else {
		fmt.Printf("✅ Saved analysis report (%d products) to data/analysis_report.json\n", len(report))
	}

	// --- Triage Engine: extract and persist review queue ---
	var reviewQueue []models.Analysis
	for _, item := range report {
		if item.NeedsReview {
			reviewQueue = append(reviewQueue, item)
		}
	}

	reviewPath := filepath.Join("data", "needs_review.json")
	reviewJSON, err := json.MarshalIndent(reviewQueue, "", "  ")
	if err != nil {
		fmt.Printf("⚠️ Error marshalling review queue: %v\n", err)
	} else {
		if err := os.WriteFile(reviewPath, reviewJSON, 0644); err != nil {
			fmt.Printf("⚠️ Error saving review queue: %v\n", err)
		} else {
			fmt.Printf("🔍 Saved review queue (%d flagged) to data/needs_review.json\n", len(reviewQueue))
		}
	}

	printTable(report)

	if *audit {
		fmt.Print(parser.FormatAuditReport(auditResults))
	}
}

func printTable(data []models.Analysis) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\nRANK\tVENDOR\tPRODUCT (Truncated)\tTYPE\tPRICE\tGRAMS\t$/GRAM\tTRUE COST (Eff.)")
	fmt.Fprintln(w, "----\t------\t-------------------\t-----\t-----\t-----\t------\t----------------")

	for i, row := range data {
		shortName := row.Name
		/*
		if len(shortName) > 30 {
			shortName = shortName[:27] + "..."
		}
		*/

		reset := "\033[0m"
		color := reset
		
		// Color logic based on Effective Cost now
		if row.EffectiveCost < 0.5 {
			color = "\033[31m" // Suspiciously cheap?
		} else if row.EffectiveCost < 1.0 {
			color = "\033[32m" // Great Deal
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t$%.2f\t%.1fg\t$%.2f\t%s$%.2f%s\n",
			i+1, row.Vendor, shortName, row.Type, row.Price, row.TotalGrams, row.CostPerGram, color, row.EffectiveCost, reset)
	}
	w.Flush()
}