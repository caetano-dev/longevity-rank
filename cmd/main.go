package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime/pprof"
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

func main() {
	refresh := flag.Bool("refresh", false, "Scrape websites to update local data")
	cpuprofile := flag.String("cpuprofile", "", "Write cpu profile to `file`")
	pprofFlag := flag.Bool("pprof", false, "Start pprof HTTP server on :6060")
	audit := flag.Bool("audit", false, "Detect products that need manual overrides in vendor_rules.json")
	supplements := flag.String("supplements", "nmn,nad,tmg,trimethylglycine,resveratrol,creatine", "Comma-separated list of supplement keywords to track")
	flag.Parse()

	if *pprofFlag {
		go func() {
			fmt.Println("📊 Profiling server started at http://localhost:6060/debug/pprof/")
			if err := http.ListenAndServe("localhost:6060", nil); err != nil {
				log.Printf("Could not start pprof server: %v", err)
			}
		}()
	}

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	if err := storage.EnsureDataDir(); err != nil {
		panic(err)
	}

	// Load vendor rules (no global state — returned explicitly)
	rulesPath := filepath.Join("data", "vendor_rules.json")
	reg, err := rules.LoadRules(rulesPath)
	if err != nil {
		fmt.Printf("⚠️ Warning: Could not load rules (%v). Running without filters.\n", err)
	} else {
		fmt.Println("✅ Loaded vendor rules from JSON")
	}

	// Build analyzer with injected dependencies
	analyzer := &parser.Analyzer{
		Rules:       reg,
		Supplements: parseSupplements(*supplements),
	}

	// Scrape or load all vendors concurrently
	vendors := config.GetVendors()
	vendorProducts := scrapeAll(vendors, reg, *refresh)

	// Analyze and optionally audit
	var report []models.Analysis
	var auditResults []parser.AuditResult

	for _, vp := range vendorProducts {
		if analyses := analyzer.AnalyzeProduct(vp.Vendor, vp.Product); analyses != nil {
			report = append(report, analyses...)
		}
		if *audit {
			if gap := analyzer.AuditProduct(vp.Vendor, vp.Product); gap != nil {
				auditResults = append(auditResults, *gap)
			}
		}
	}

	// Sort by effective cost (true value)
	sort.Slice(report, func(i, j int) bool {
		return report[i].EffectiveCost < report[j].EffectiveCost
	})

	if err := storage.SaveJSON(filepath.Join("data", "analysis_report.json"), report); err != nil {
		fmt.Printf("⚠️ Error saving analysis report: %v\n", err)
	} else {
		fmt.Printf("✅ Saved analysis report (%d products) to data/analysis_report.json\n", len(report))
	}

	saveReviewQueue(report)
	printTable(report)

	if *audit {
		fmt.Print(parser.FormatAuditReport(auditResults))
	}
}

// parseSupplements splits a comma-separated string into a cleaned keyword list.
func parseSupplements(raw string) []string {
	if raw == "" {
		return []string{"nmn", "nad", "tmg", "trimethylglycine", "resveratrol", "creatine"}
	}
	var cleaned []string
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(strings.ToLower(s))
		if s != "" {
			cleaned = append(cleaned, s)
		}
	}
	return cleaned
}

// vendorProduct pairs a vendor name with a single filtered product.
type vendorProduct struct {
	Vendor  string
	Product models.Product
}

// scrapeAll fetches or loads products for all vendors concurrently, applies
// blocklist rules, and returns the flattened list of vendor+product pairs.
func scrapeAll(vendors []models.Vendor, reg rules.Registry, refresh bool) []vendorProduct {
	type result struct {
		VendorName string
		Products   []models.Product
		Err        error
	}

	ch := make(chan result, len(vendors))
	var wg sync.WaitGroup

	for _, v := range vendors {
		wg.Add(1)
		go func(v models.Vendor) {
			defer wg.Done()
			products, err := scrapeOrLoad(v, refresh)
			ch <- result{VendorName: v.Name, Products: products, Err: err}
		}(v)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var all []vendorProduct
	for res := range ch {
		if res.Err != nil {
			fmt.Printf("❌ Error for %s: %v\n", res.VendorName, res.Err)
			continue
		}
		for _, p := range res.Products {
			if rules.ApplyRules(reg, res.VendorName, &p) {
				all = append(all, vendorProduct{Vendor: res.VendorName, Product: p})
			}
		}
	}
	return all
}

// scrapeOrLoad either scrapes fresh data or loads from the local JSON cache.
func scrapeOrLoad(v models.Vendor, refresh bool) ([]models.Product, error) {
	shouldScrape := refresh
	if !shouldScrape {
		if _, err := os.Stat(storage.VendorFilename(v.Name)); os.IsNotExist(err) {
			shouldScrape = true
		}
	}

	// Cloudflare-blocked vendors rely on manually-maintained JSON
	if shouldScrape && v.Cloudflare {
		fmt.Printf("🛡️  Skipping %s (Cloudflare-protected). Using local JSON if available.\n", v.Name)
		shouldScrape = false
	}

	if !shouldScrape {
		return storage.LoadJSON[[]models.Product](storage.VendorFilename(v.Name))
	}

	products, err := scraper.FetchProducts(v)
	if err != nil {
		return nil, fmt.Errorf("scraping: %w", err)
	}

	if err := storage.SaveJSON(storage.VendorFilename(v.Name), products); err != nil {
		fmt.Printf("⚠️ Error saving data for %s: %v\n", v.Name, err)
	} else {
		fmt.Printf("✅ Saved %d products for %s\n", len(products), v.Name)
	}

	return products, nil
}

// saveReviewQueue extracts flagged products and persists them.
func saveReviewQueue(report []models.Analysis) {
	var queue []models.Analysis
	for _, item := range report {
		if item.NeedsReview {
			queue = append(queue, item)
		}
	}

	path := filepath.Join("data", "needs_review.json")
	data, err := json.MarshalIndent(queue, "", "  ")
	if err != nil {
		fmt.Printf("⚠️ Error marshalling review queue: %v\n", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Printf("⚠️ Error saving review queue: %v\n", err)
		return
	}
	fmt.Printf("🔍 Saved review queue (%d flagged) to data/needs_review.json\n", len(queue))
}

func printTable(data []models.Analysis) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\nRANK\tVENDOR\tPRODUCT (Truncated)\tTYPE\tPRICE\tACTIVE g\tGROSS g\t$/GRAM\tTRUE COST (Eff.)")
	fmt.Fprintln(w, "----\t------\t-------------------\t-----\t-----\t--------\t-------\t------\t----------------")

	const (
		reset = "\033[0m"
		red   = "\033[31m"
		green = "\033[32m"
	)

	for i, row := range data {
		color := reset
		if row.EffectiveCost < 0.5 {
			color = red
		} else if row.EffectiveCost < 1.0 {
			color = green
		}

		grossCol := "—"
		if row.GrossGrams > 0 {
			grossCol = fmt.Sprintf("%.1fg", row.GrossGrams)
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t$%.2f\t%.1fg\t%s\t$%.2f\t%s$%.2f%s\n",
			i+1, row.Vendor, row.Name, row.Type, row.Price, row.ActiveGrams, grossCol, row.CostPerGram, color, row.EffectiveCost, reset)
	}
	w.Flush()
}