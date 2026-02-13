import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "export",
  images: {
    remotePatterns: [
      {
        protocol: "https",
        hostname: "cdn.shopify.com",
        pathname: "/s/files/**",
      },
      {
        protocol: "https",
        hostname: "donotage.org",
        pathname: "/media/**",
      },
      {
        protocol: "https",
        hostname: "renuebyscience.com",
        pathname: "/**",
      },
      {
        protocol: "https",
        hostname: "getwonderfeel.com",
        pathname: "/**",
      },
      {
        protocol: "https",
        hostname: "www.jinfiniti.com",
        pathname: "/**",
      },
      {
        protocol: "https",
        hostname: "nutricost.com",
        pathname: "/**",
      },
      {
        protocol: "https",
        hostname: "www.prohealth.com",
        pathname: "/**",
      },
    ],
    unoptimized: true,
  },
};

export default nextConfig;