"use client";

const SUPPLEMENT_OPTIONS = [
  { label: "All", value: "all" },
  { label: "NMN", value: "nmn" },
  { label: "NAD+", value: "nad" },
  { label: "TMG", value: "tmg" },
  { label: "Resveratrol", value: "resveratrol" },
  { label: "Creatine", value: "creatine" },
] as const;

export type SupplementFilter = (typeof SUPPLEMENT_OPTIONS)[number]["value"];

interface SupplementFilterProps {
  active: SupplementFilter;
  onChange: (value: SupplementFilter) => void;
}

export default function SupplementFilter({
  active,
  onChange,
}: SupplementFilterProps) {
  return (
    <div className="flex flex-wrap gap-2">
      {SUPPLEMENT_OPTIONS.map((opt) => {
        const isActive = active === opt.value;
        return (
          <button
            key={opt.value}
            onClick={() => onChange(opt.value)}
            className={`rounded-full px-4 py-1.5 text-sm font-medium transition-all duration-150 cursor-pointer
              ${
                isActive
                  ? "bg-emerald-600 text-white shadow-md shadow-emerald-600/25"
                  : "bg-zinc-800 text-zinc-400 hover:bg-zinc-700 hover:text-zinc-200"
              }`}
          >
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}

export { SUPPLEMENT_OPTIONS };