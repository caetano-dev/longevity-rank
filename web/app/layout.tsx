import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Longevity Rank — Cheapest NMN & NAD+ Supplements Compared",
  description:
    "Real-time price comparison of NMN, NAD+, TMG, Resveratrol & Creatine supplements. Bioavailability-adjusted True Cost rankings updated daily.",
  keywords: [
    "NMN",
    "NAD+",
    "longevity supplements",
    "cheapest NMN",
    "NMN price comparison",
    "TMG",
    "resveratrol",
    "creatine",
    "bioavailability",
    "anti-aging",
  ],
  openGraph: {
    title: "Longevity Rank — Cheapest NMN & NAD+ Supplements",
    description:
      "Who has the cheapest authentic NMN today? Bioavailability-adjusted rankings, updated daily.",
    type: "website",
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className="dark">
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link
          rel="preconnect"
          href="https://fonts.gstatic.com"
          crossOrigin="anonymous"
        />
        <link
          href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&family=JetBrains+Mono:wght@400;500;600&display=swap"
          rel="stylesheet"
        />
      </head>
      <body className="min-h-screen antialiased">{children}</body>
    </html>
  );
}