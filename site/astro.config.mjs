import { defineConfig } from "astro/config";
import sitemap from "@astrojs/sitemap";

export default defineConfig({
  site: "https://wilsonnet.github.io",
  base: "/lofi-hiphopper",
  output: "static",
  trailingSlash: "always",
  integrations: [sitemap()],
  compressHTML: true,
});
