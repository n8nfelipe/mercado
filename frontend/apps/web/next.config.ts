import path from "node:path";
import type { NextConfig } from "next";

const workspaceRoot = path.join(process.cwd(), "../../..");

const nextConfig: NextConfig = {
  output: "standalone",
  outputFileTracingRoot: workspaceRoot,
  turbopack: {
    root: workspaceRoot,
  },
};

export default nextConfig;
