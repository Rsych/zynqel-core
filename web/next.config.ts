import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "export",
  basePath: "/console",
  images: {
    unoptimized: true,
  },
};

export default nextConfig;
