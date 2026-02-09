package main

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"longevity-ranker/internal/config"
	"longevity-ranker/internal/parser"
	"longevity-ranker/internal/scraper"
	"longevity-ranker/internal/models"
)

func main() {
	vendors := config.GetVendors()
	var report []models.Analysis

	// 1. Scrape all vendors concurrently (sequential for now for simplicity)
	for _, v := range vendors {
		products, err := scraper.FetchProducts(v)
		if err != nil {
			fmt.Printf("Error scraping %s: %v\n", v.Name, err)
			continue
		}

		// 2. Analyze products
		for _, p := range products {
			analysis := parser.AnalyzeProduct(v.Name, p)
			if analysis != nil {
				report = append(report, *analysis)
			}
		}
	}

	// 3. Sort by ROI (Cost Per Gram - Cheapest first)
	sort.Slice(report, func(i, j int) bool {
		return report[i].CostPerGram < report[j].CostPerGram
	})

	// 4. Print Table
	printTable(report)
}

func printTable(data []models.Analysis) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\nRANK\tVENDOR\tPRODUCT (Truncated)\tTYPE\tPRICE\tGRAMS\t$/GRAM")
	fmt.Fprintln(w, "----\t------\t-------------------\t-----\t-----\t-----\t------")

	for i, row := range data {
		// Truncate long titles
		shortName := row.Name
		if len(shortName) > 30 {
			shortName = shortName[:27] + "..."
		}

		// Colorize output (Green for cheap, Red for expensive)
		reset := "\033[0m"
		color := reset
		if row.CostPerGram < 1.0 {
			color = "\033[31m" // Red (Suspiciously cheap)
		} else if row.CostPerGram < 2.5 {
			color = "\033[32m" // Green (Great Deal)
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t$%.2f\t%.1fg\t%s$%.2f%s\n",
			i+1, row.Vendor, shortName, row.Type, row.Price, row.TotalGrams, color, row.CostPerGram, reset)
	}
	w.Flush()
}