import { readFileSync, existsSync, statSync } from "node:fs";
import { join, resolve } from "node:path";

const dist = resolve("dist");
const failures = [];
const html = readFileSync(join(dist, "index.html"), "utf8");
const required = [
  ["title", /<title>[^<]+<\/title>/],
  ["description", /<meta name="description" content="[^"]{60,}"/],
  ["canonical", /<link rel="canonical" href="https:\/\/wilsonnet\.github\.io\/lofi-hiphopper\/"/],
  ["viewport", /<meta name="viewport"/],
  ["open graph image", /<meta property="og:image" content="[^"]+"/],
  ["twitter card", /<meta name="twitter:card" content="summary_large_image"/],
  ["h1", /<h1[ >]/],
  ["json-ld", /<script type="application\/ld\+json">/],
];
for (const [name, pattern] of required) {
  if (!pattern.test(html)) failures.push(`missing ${name}`);
}
if ((html.match(/<h1[ >]/g) || []).length !== 1) failures.push("expected exactly one h1");
const ld = html.match(/<script type="application\/ld\+json">([\s\S]*?)<\/script>/);
if (ld) {
  try {
    const data = JSON.parse(ld[1]);
    if (data["@type"] !== "SoftwareApplication") failures.push("JSON-LD is not SoftwareApplication");
    if (!data.url || !data.downloadUrl) failures.push("JSON-LD missing URL fields");
  } catch (error) {
    failures.push(`invalid JSON-LD: ${error.message}`);
  }
}
const images = [...html.matchAll(/<img\b[^>]*>/g)].map((match) => match[0]);
for (const img of images) {
  if (!/\balt="/.test(img)) failures.push("image missing alt text");
}

const files = ["robots.txt", "site.webmanifest", "sitemap-index.xml", "sitemap-0.xml", "assets/og.png", "assets/visualizers.png", "assets/shortcuts.png", "assets/favicon.svg"];
for (const file of files) {
  const path = join(dist, file);
  if (!existsSync(path) || statSync(path).size === 0) failures.push(`missing or empty ${file}`);
}
const robots = existsSync(join(dist, "robots.txt")) ? readFileSync(join(dist, "robots.txt"), "utf8") : "";
if (!robots.includes("sitemap-index.xml")) failures.push("robots.txt does not reference the sitemap");
const sitemap = existsSync(join(dist, "sitemap-0.xml")) ? readFileSync(join(dist, "sitemap-0.xml"), "utf8") : "";
if (!sitemap.includes("https://wilsonnet.github.io/lofi-hiphopper/")) failures.push("sitemap misses the homepage");

const urls = [...html.matchAll(/\b(?:src|href)="([^"]+)"/g)].map((match) => match[1]);
for (const url of urls) {
  if (!url || url.startsWith("#") || /^(https?:|mailto:|data:)/.test(url)) continue;
  let file = url.split("#")[0].split("?")[0];
  if (file.startsWith("/lofi-hiphopper/")) file = file.slice("/lofi-hiphopper/".length);
  const path = file.startsWith("/") ? join(dist, file) : join(dist, file);
  if (!existsSync(path)) failures.push(`broken local asset: ${url}`);
}

if (failures.length) {
  console.error("site check: FAIL");
  for (const failure of failures) console.error("  " + failure);
  process.exit(1);
}
console.log(`site check: PASS (${images.length} images, ${urls.length} links)`);
