package main

import (
	"encoding/json"
	_ "net/http/pprof"
	"runtime/pprof" 
	"net/http"
	"log"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"

	"longevity-ranker/internal/config"
	"longevity-ranker/internal/models"
	"longevity-ranker/internal/parser"
	"longevity-ranker/internal/rules"
	"longevity-ranker/internal/scraper"
	"longevity-ranker/internal/storage"
)

// vendorResult holds the scraped/loaded products for a single vendor.
// Collected via channel from concurrent goroutines.
type vendorResult struct {
	VendorName string
	Products   []models.Product
	Err        error
}

func main() {
	go func() {
        fmt.Println("📊 Profiling server started at http://localhost:6060/debug/pprof/")
        if err := http.ListenAndServe("localhost:6060", nil); err != nil {
            log.Printf("Could not start pprof server: %v", err)
        }
    }()
	
	refresh := flag.Bool("refresh", false, "Scrape websites to update local data")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to `file`")
	audit := flag.Bool("audit", false, "Detect products that need manual overrides in vendor_rules.json")
	supplements := flag.String("supplements", "nmn,nad,tmg,trimethylglycine,resveratrol,creatine", "Comma-separated list of supplement keywords to track")
	flag.Parse()
	
	if *cpuprofile != "" {
			f, err := os.Create(*cpuprofile)
			if err != nil {
				log.Fatal("could not create CPU profile: ", err)
			}
			defer f.Close() // Ensure file is closed at the end
			if err := pprof.StartCPUProfile(f); err != nil {
				log.Fatal("could not start CPU profile: ", err)
			}
			defer pprof.StopCPUProfile() 
		}
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

	// --- Concurrent scraping/loading via goroutines ---
	resultsCh := make(chan vendorResult, len(vendors))
	var wg sync.WaitGroup

	for _, v := range vendors {
		wg.Add(1)
		go func(v models.Vendor) {
			defer wg.Done()

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

			var products []models.Product
			var err error

			if shouldScrape {
				products, err = scraper.FetchProducts(v)
				if err != nil {
					resultsCh <- vendorResult{VendorName: v.Name, Err: fmt.Errorf("scraping: %w", err)}
					return
				}
				if saveErr := storage.SaveProducts(v.Name, products); saveErr != nil {
					fmt.Printf("⚠️ Error saving data for %s: %v\n", v.Name, saveErr)
				}
				fmt.Printf("✅ Saved %d products for %s\n", len(products), v.Name)
			} else {
				products, err = storage.LoadProducts(v.Name)
				if err != nil {
					resultsCh <- vendorResult{VendorName: v.Name, Err: fmt.Errorf("loading: %w", err)}
					return
				}
			}

			resultsCh <- vendorResult{VendorName: v.Name, Products: products}
		}(v)
	}

	// Close channel once all goroutines complete
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	// --- Collect results from channel ---
	var allProducts []struct {
		VendorName string
		Product    models.Product
	}

	for res := range resultsCh {
		if res.Err != nil {
			fmt.Printf("❌ Error for %s: %v\n", res.VendorName, res.Err)
			continue
		}

		for _, p := range res.Products {
			keep := rules.ApplyRules(res.VendorName, &p)
			if !keep {
				continue
			}

			allProducts = append(allProducts, struct {
				VendorName string
				Product    models.Product
			}{res.VendorName, p})
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
	fmt.Fprintln(w, "\nRANK\tVENDOR\tPRODUCT (Truncated)\tTYPE\tPRICE\tACTIVE g\tGROSS g\t$/GRAM\tTRUE COST (Eff.)")
	fmt.Fprintln(w, "----\t------\t-------------------\t-----\t-----\t--------\t-------\t------\t----------------")

	for i, row := range data {
		shortName := row.Name

		reset := "\033[0m"
		color := reset

		// Color logic based on Effective Cost now
		if row.EffectiveCost < 0.5 {
			color = "\033[31m" // Suspiciously cheap?
		} else if row.EffectiveCost < 1.0 {
			color = "\033[32m" // Great Deal
		}

		// Show GrossGrams whenever it is non-zero; only show — when truly 0
		// (the correct state for Capsules/Tablets that don't advertise gross weight).
		grossCol := "—"
		if row.GrossGrams > 0 {
			grossCol = fmt.Sprintf("%.1fg", row.GrossGrams)
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t$%.2f\t%.1fg\t%s\t$%.2f\t%s$%.2f%s\n",
			i+1, row.Vendor, shortName, row.Type, row.Price, row.ActiveGrams, grossCol, row.CostPerGram, color, row.EffectiveCost, reset)
	}
	w.Flush()
}