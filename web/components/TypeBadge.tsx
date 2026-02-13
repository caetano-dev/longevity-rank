interface TypeBadgeProps {
  type: string;
}

const TYPE_CONFIG: Record<string, { className: string; label: string }> = {
  Capsules: { className: "badge-capsules", label: "Capsules" },
  Powder: { className: "badge-powder", label: "Powder" },
  Tablets: { className: "badge-tablets", label: "Tablets" },
  Gel: { className: "badge-gel", label: "Gel" },
  "Multi-Pack": { className: "badge-multipack", label: "Multi-Pack" },
  "Hybrid Bundle": { className: "badge-hybrid", label: "Hybrid" },
  Single: { className: "badge-single", label: "Single" },
};

export default function TypeBadge({ type }: TypeBadgeProps) {
  const config = TYPE_CONFIG[type] ?? {
    className: "badge-single",
    label: type,
  };

  return <span className={`badge ${config.className}`}>{config.label}</span>;
}