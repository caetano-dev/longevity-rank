import { loadReport } from "@/lib/data";
import type { AnalysisWithVendorInfo } from "@/components/ProductTable";
import ProductTable from "@/components/ProductTable";
import vendors from "@/lib/vendors";

export const dynamic = "force-static";

const AFFILIATE_ID = process.env.AFFILIATE_ID ?? "";

export default function Home() {
  // loadReport() reads data/analysis_report.json and maps snake_case → camelCase.
  // No parsing, regex, or math — the Go backend did all of that.
  const report = loadReport();

  // Build a lookup map from vendor name to VendorInfo for affiliate links
  const vendorMap = new Map(vendors.map((v) => [v.name, v]));

  // Attach vendorInfo to each analysis for affiliate link construction
  const enriched: AnalysisWithVendorInfo[] = report
    .map((a) => {
      const vendorInfo = vendorMap.get(a.vendor);
      if (!vendorInfo) return null;
      return { ...a, vendorInfo };
    })
    .filter((a): a is AnalysisWithVendorInfo => a !== null);

  const productCount = enriched.length;
  const vendorCount = new Set(enriched.map((a) => a.vendor)).size;

  return (
    <>
      {/* Hero */}
      <section className="relative overflow-hidden border-b border-zinc-800 bg-gradient-to-b from-zinc-900 to-[var(--color-brand-900)]">
        <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_top,rgba(16,185,129,0.08),transparent_60%)]" />
        <div className="relative mx-auto max-w-7xl px-4 py-16 sm:px-6 sm:py-24 lg:px-8">
          <div className="text-center">
            <h1 className="text-4xl font-extrabold tracking-tight text-white sm:text-5xl lg:text-6xl">
              Longevity{" "}
              <span className="bg-gradient-to-r from-emerald-400 to-teal-400 bg-clip-text text-transparent">
                Ranker
              </span>
            </h1>
            <p className="mx-auto mt-4 max-w-2xl text-lg text-zinc-400 sm:text-xl">
              Who has the cheapest authentic NMN &amp; longevity supplements today?
              <br className="hidden sm:block" />
              Prices updated daily. Ranked by{" "}
              <span className="font-semibold text-emerald-400">True Cost</span>{" "}
              (bioavailability-adjusted $/gram).
            </p>
            <div className="mt-8 flex items-center justify-center gap-6 text-sm text-zinc-500">
              <div className="flex items-center gap-1.5">
                <span className="inline-block h-2 w-2 rounded-full bg-emerald-500 animate-pulse" />
                <span>
                  <strong className="text-zinc-300">{vendorCount}</strong> vendors
                </span>
              </div>
              <div className="h-4 w-px bg-zinc-700" />
              <div>
                <strong className="text-zinc-300">{productCount}</strong> products ranked
              </div>
              <div className="h-4 w-px bg-zinc-700" />
              <div>Updated daily</div>
            </div>
          </div>
        </div>
      </section>

      {/* How True Cost works */}
      <section className="border-b border-zinc-800 bg-zinc-900/30">
        <div className="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8">
          <div className="grid gap-6 sm:grid-cols-3 text-center">
            <div className="rounded-xl border border-zinc-800 bg-zinc-900/50 p-5">
              <div className="mb-2 text-2xl">💊</div>
              <h3 className="text-sm font-semibold text-zinc-200">Raw $/Gram</h3>
              <p className="mt-1 text-xs text-zinc-500">
                Price divided by total active ingredient weight. The simple metric.
              </p>
            </div>
            <div className="rounded-xl border border-zinc-800 bg-zinc-900/50 p-5">
              <div className="mb-2 text-2xl">🧬</div>
              <h3 className="text-sm font-semibold text-zinc-200">
                Bioavailability Multiplier
              </h3>
              <p className="mt-1 text-xs text-zinc-500">
                Liposomal = 1.5×, Sublingual/Gel/Tablets = 1.1×. Rewards better delivery.
              </p>
            </div>
            <div className="rounded-xl border border-emerald-800/50 bg-emerald-950/20 p-5">
              <div className="mb-2 text-2xl">🏆</div>
              <h3 className="text-sm font-semibold text-emerald-400">True Cost</h3>
              <p className="mt-1 text-xs text-zinc-500">
                $/Gram ÷ Multiplier. The real price per gram your body actually absorbs.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* Product Table */}
      <section className="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8">
        <ProductTable analyses={enriched} affiliateId={AFFILIATE_ID || undefined} />
      </section>

      {/* FAQ */}
      <section className="border-t border-zinc-800 bg-zinc-900/30">
        <div className="mx-auto max-w-3xl px-4 py-12 sm:px-6 lg:px-8">
          <h2 className="mb-8 text-center text-xl font-bold text-zinc-200">
            Frequently Asked Questions
          </h2>
          <div className="space-y-6">
            <details className="group rounded-xl border border-zinc-800 bg-zinc-900/50 p-4">
              <summary className="cursor-pointer text-sm font-medium text-zinc-300 group-open:text-emerald-400">
                What is &ldquo;True Cost&rdquo;?
              </summary>
              <p className="mt-2 text-sm text-zinc-500">
                True Cost divides the raw $/gram by a bioavailability multiplier.
                Liposomal products get a 1.5× bonus because more active ingredient
                reaches your bloodstream. Sublingual, gel, and tablet forms get 1.1×.
                Standard capsules and powders get 1.0×.
              </p>
            </details>
            <details className="group rounded-xl border border-zinc-800 bg-zinc-900/50 p-4">
              <summary className="cursor-pointer text-sm font-medium text-zinc-300 group-open:text-emerald-400">
                How often are prices updated?
              </summary>
              <p className="mt-2 text-sm text-zinc-500">
                A Go scraper runs daily via GitHub Actions at 08:00 UTC. If any prices
                change, the site rebuilds automatically. Some Cloudflare-protected
                vendors (Jinfiniti, Wonderfeel) are updated manually.
              </p>
            </details>
            <details className="group rounded-xl border border-zinc-800 bg-zinc-900/50 p-4">
              <summary className="cursor-pointer text-sm font-medium text-zinc-300 group-open:text-emerald-400">
                Why are some vendors missing products?
              </summary>
              <p className="mt-2 text-sm text-zinc-500">
                Products are filtered by supplement type (NMN, NAD+, TMG, Resveratrol,
                Creatine). Items that don&rsquo;t match these keywords, or that appear on a
                vendor&rsquo;s blocklist (bundles, non-supplement SKUs), are excluded.
              </p>
            </details>
          </div>
        </div>
      </section>

      {/* Footer — FDA, EU, Affiliate Disclosures */}
      <footer className="border-t border-zinc-800 bg-zinc-950">
        <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
          <div className="space-y-4 text-center text-xs leading-relaxed text-zinc-600">
            <p>
              <strong className="text-zinc-500">FDA Disclaimer:</strong> These
              statements have not been evaluated by the Food and Drug
              Administration. Products listed on this site are not intended to
              diagnose, treat, cure, or prevent any disease. This site is for
              informational purposes only and does not constitute medical advice.
              Consult your healthcare provider before starting any supplement regimen.
            </p>
            <p>
              <strong className="text-zinc-500">EU Notice:</strong> NMN is
              classified as a Novel Food in the European Union. Listings are
              provided for research and personal import purposes only. Check
              your local regulations before purchasing.
            </p>
            <p>
              <strong className="text-zinc-500">Affiliate Disclosure:</strong>{" "}
              This site may earn a commission from qualifying purchases at no
              extra cost to you. All rankings are based solely on
              bioavailability-adjusted cost-per-gram calculations.
            </p>
            <div className="pt-4 border-t border-zinc-800/40">
              <p className="text-zinc-700">
                Longevity Ranker &middot; Data sourced from public vendor storefronts &middot;{" "}
                Prices updated daily via automated scraping &middot;{" "}
              </p>
            </div>
          </div>
        </div>
      </footer>
    </>
  );
}