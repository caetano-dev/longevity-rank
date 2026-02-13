interface RankBadgeProps {
  rank: number;
}

export default function RankBadge({ rank }: RankBadgeProps) {
  if (rank <= 3) {
    const className =
      rank === 1 ? "rank-1" : rank === 2 ? "rank-2" : "rank-3";

    const emoji = rank === 1 ? "🥇" : rank === 2 ? "🥈" : "🥉";

    return (
      <span
        className={`${className} inline-flex h-8 w-8 items-center justify-center rounded-full text-sm`}
      >
        {emoji}
      </span>
    );
  }

  return (
    <span className="inline-flex h-8 w-8 items-center justify-center rounded-full bg-zinc-800 text-sm font-medium text-zinc-400">
      {rank}
    </span>
  );
}