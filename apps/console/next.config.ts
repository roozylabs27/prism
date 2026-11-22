import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  // Use standalone output for Docker builds, but omit it on Vercel where output tracing is handled natively
  output: process.env.VERCEL ? undefined : 'standalone',
  reactStrictMode: true,
};

export default nextConfig;
