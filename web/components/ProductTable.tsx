"use client";

import { useState, useMemo } from "react";
import type { Analysis } from "@/lib/types";
import type { VendorInfo } from "@/lib/vendors";
import { buildAffiliateUrl } from "@/lib/vendors";
import RankBadge from "./RankBadge";
import TypeBadge from "./TypeBadge";
import SupplementFilter, {
  type SupplementFilter as FilterValue,
} from "./SupplementFilter";

interface AnalysisWithVendorInfo extends Analysis {
  vendorInfo: VendorInfo;
}

interface ProductTableProps {
  /** All analyses pre-sorted by effectiveCost ascending, with vendorInfo attached */
  analyses: AnalysisWithVendorInfo[];
  affiliateId?: string;
}

/** Maps supplement filter values to the keywords the analyzer uses */
const FILTER_KEYWORDS: Record<string, string[]> = {
  all: [],
  nmn: ["nmn"],
  nad: ["nad"],
  tmg: ["tmg", "trimethylglycine"],
  resveratrol: ["resveratrol"],
  creatine: ["creatine"],
};

function formatCurrency(value: number): string {
  return `$${value.toFixed(2)}`;
}

function formatGrams(value: number): string {
  if (value >= 1) {
    return `${value.toFixed(1)}g`;
  }
  return `${(value * 1000).toFixed(0)}mg`;
}

function formatCostPerGram(value: number): string {
  return `$${value.toFixed(2)}/g`;
}

function ProductImage({ src, alt }: { src: string; alt: string }) {
  if (!src) {
    return (
      <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-zinc-800 text-xs text-zinc-500">
        N/A
      </div>
    );
  }
  return (
    <img
      src={src}
      alt={alt}
      width={48}
      height={48}
      className="h-12 w-12 rounded-lg object-cover bg-zinc-800"
      loading="lazy"
    />
  );
}

function matchesFilter(analysis: AnalysisWithVendorInfo, filter: FilterValue): boolean {
  if (filter === "all") return true;
  const keywords = FILTER_KEYWORDS[filter] ?? [];
  const searchStr = (analysis.name + " " + analysis.handle + " " + analysis.vendor).toLowerCase();
  return keywords.some((kw) => searchStr.includes(kw));
}

export default function ProductTable({ analyses, affiliateId }: ProductTableProps) {
  const [filter, setFilter] = useState<FilterValue>("all");
  const [sortBy, setSortBy] = useState<"effectiveCost" | "costPerGram" | "price">("effectiveCost");
  const [sortAsc, setSortAsc] = useState(true);

  const filtered = useMemo(() => {
    const items = analyses.filter((a) => matchesFilter(a, filter));

    items.sort((a, b) => {
      const va = a[sortBy];
      const vb = b[sortBy];
      return sortAsc ? va - vb : vb - va;
    });

    return items;
  }, [analyses, filter, sortBy, sortAsc]);

  function handleSort(column: "effectiveCost" | "costPerGram" | "price") {
    if (sortBy === column) {
      setSortAsc(!sortAsc);
    } else {
      setSortBy(column);
      setSortAsc(true);
    }
  }

  function SortIndicator({ column }: { column: "effectiveCost" | "costPerGram" | "price" }) {
    if (sortBy !== column) return <span className="ml-1 text-zinc-600">↕</span>;
    return <span className="ml-1 text-emerald-400">{sortAsc ? "↑" : "↓"}</span>;
  }

  const bestEffectiveCost = filtered.length > 0 ? filtered[0].effectiveCost : 0;

  return (
    <div className="w-full">
      {/* Filter bar */}
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <SupplementFilter active={filter} onChange={setFilter} />
        <p className="text-sm text-zinc-500">
          {filtered.length} product{filtered.length !== 1 ? "s" : ""} found
        </p>
      </div>

      {filtered.length === 0 && (
        <div className="rounded-xl border border-zinc-800 bg-zinc-900/50 py-16 text-center">
          <p className="text-lg text-zinc-400">No products match this filter.</p>
          <p className="mt-1 text-sm text-zinc-600">Try selecting a different supplement type.</p>
        </div>
      )}

      {/* Desktop Table (hidden below md) */}
      {filtered.length > 0 && (
        <div className="hidden md:block overflow-x-auto custom-scrollbar rounded-xl border border-zinc-800 bg-zinc-900/50">
          <table className="w-full min-w-[900px] text-sm">
            <thead className="sticky-header">
              <tr className="border-b border-zinc-800 text-left text-xs uppercase tracking-wider text-zinc-500">
                <th className="px-4 py-3 w-12">#</th>
                <th className="px-4 py-3 w-14">Image</th>
                <th className="px-4 py-3">Vendor</th>
                <th className="px-4 py-3">Product</th>
                <th className="px-4 py-3 w-24">Type</th>
                <th
                  className="px-4 py-3 w-24 cursor-pointer select-none hover:text-zinc-300 text-right"
                  onClick={() => handleSort("price")}
                >
                  Price
                  <SortIndicator column="price" />
                </th>
                <th className="px-4 py-3 w-20 text-right">Grams</th>
                <th
                  className="px-4 py-3 w-24 cursor-pointer select-none hover:text-zinc-300 text-right"
                  onClick={() => handleSort("costPerGram")}
                >
                  $/Gram
                  <SortIndicator column="costPerGram" />
                </th>
                <th
                  className="px-4 py-3 w-28 cursor-pointer select-none hover:text-zinc-300 text-right"
                  onClick={() => handleSort("effectiveCost")}
                >
                  True Cost
                  <SortIndicator column="effectiveCost" />
                </th>
                <th className="px-4 py-3 w-20"></th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((item, idx) => {
                const rank = idx + 1;
                const href = buildAffiliateUrl(item.vendorInfo, item.handle, affiliateId);
                const isBest = item.effectiveCost === bestEffectiveCost && sortBy === "effectiveCost" && sortAsc;

                return (
                  <tr
                    key={`${item.vendor}-${item.handle}-${idx}`}
                    className={`table-row-hover border-b border-zinc-800/50 ${
                      isBest ? "bg-emerald-950/20" : ""
                    }`}
                  >
                    <td className="px-4 py-3">
                      <RankBadge rank={rank} />
                    </td>
                    <td className="px-4 py-3">
                      <ProductImage src={item.imageURL} alt={item.name} />
                    </td>
                    <td className="px-4 py-3">
                      <span className="font-medium text-zinc-300">{item.vendor}</span>
                    </td>
                    <td className="px-4 py-3">
                      <span className="text-zinc-200 line-clamp-2" title={item.name}>
                        {item.name}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <TypeBadge type={item.type} />
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-zinc-300">
                      {formatCurrency(item.price)}
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-zinc-400">
                      {formatGrams(item.totalGrams)}
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-zinc-400">
                      {formatCostPerGram(item.costPerGram)}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <span
                        className={`font-mono font-semibold ${
                          isBest
                            ? "best-price text-emerald-400"
                            : "text-zinc-200"
                        }`}
                      >
                        {formatCostPerGram(item.effectiveCost)}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <a
                        href={href}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center rounded-lg bg-emerald-600/20 px-3 py-1.5 text-xs font-semibold text-emerald-400 transition-all hover:bg-emerald-600/30 hover:text-emerald-300"
                      >
                        Buy →
                      </a>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Mobile Cards (visible below md) */}
      {filtered.length > 0 && (
        <div className="md:hidden flex flex-col gap-3">
          {filtered.map((item, idx) => {
            const rank = idx + 1;
            const href = buildAffiliateUrl(item.vendorInfo, item.handle, affiliateId);
            const isBest = item.effectiveCost === bestEffectiveCost && sortBy === "effectiveCost" && sortAsc;

            return (
              <div
                key={`mobile-${item.vendor}-${item.handle}-${idx}`}
                className={`card-shine rounded-xl border p-4 ${
                  isBest
                    ? "border-emerald-700/50 bg-emerald-950/20"
                    : "border-zinc-800 bg-zinc-900/50"
                }`}
              >
                <div className="flex items-start gap-3">
                  {/* Rank + Image */}
                  <div className="flex flex-col items-center gap-2">
                    <RankBadge rank={rank} />
                    <ProductImage src={item.imageURL} alt={item.name} />
                  </div>

                  {/* Content */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="text-xs font-medium text-zinc-500">{item.vendor}</span>
                      <TypeBadge type={item.type} />
                    </div>
                    <p className="mt-1 text-sm font-medium text-zinc-200 line-clamp-2">
                      {item.name}
                    </p>

                    {/* Stats row */}
                    <div className="mt-3 grid grid-cols-2 gap-x-4 gap-y-1 text-xs">
                      <div>
                        <span className="text-zinc-500">Price</span>
                        <p className="font-mono font-medium text-zinc-300">
                          {formatCurrency(item.price)}
                        </p>
                      </div>
                      <div>
                        <span className="text-zinc-500">Total</span>
                        <p className="font-mono font-medium text-zinc-400">
                          {formatGrams(item.totalGrams)}
                        </p>
                      </div>
                      <div>
                        <span className="text-zinc-500">$/Gram</span>
                        <p className="font-mono font-medium text-zinc-400">
                          {formatCostPerGram(item.costPerGram)}
                        </p>
                      </div>
                      <div>
                        <span className="text-zinc-500">True Cost</span>
                        <p
                          className={`font-mono font-semibold ${
                            isBest ? "text-emerald-400" : "text-zinc-200"
                          }`}
                        >
                          {formatCostPerGram(item.effectiveCost)}
                        </p>
                      </div>
                    </div>
                  </div>
                </div>

                {/* Buy button */}
                <a
                  href={href}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="mt-3 flex w-full items-center justify-center rounded-lg bg-emerald-600/20 py-2 text-sm font-semibold text-emerald-400 transition-all hover:bg-emerald-600/30 hover:text-emerald-300"
                >
                  View Deal →
                </a>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

export type { AnalysisWithVendorInfo };