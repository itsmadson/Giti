#!/usr/bin/env node
// Fails if the en and fa dictionaries don't have identical key sets.
// Run: node scripts/i18n-check.mjs
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const here = dirname(fileURLToPath(import.meta.url));
const dictDir = join(here, "..", "src", "i18n", "dictionaries");

// Extract top-level "key": occurrences from a dictionary source file.
function keys(file) {
  const src = readFileSync(join(dictDir, file), "utf8");
  const set = new Set();
  const re = /"([^"]+)"\s*:/g;
  let m;
  while ((m = re.exec(src))) set.add(m[1]);
  return set;
}

const en = keys("en.ts");
const fa = keys("fa.ts");

const missingInFa = [...en].filter((k) => !fa.has(k));
const missingInEn = [...fa].filter((k) => !en.has(k));

if (missingInFa.length || missingInEn.length) {
  if (missingInFa.length) console.error("Missing in fa.ts:", missingInFa.join(", "));
  if (missingInEn.length) console.error("Missing in en.ts:", missingInEn.join(", "));
  process.exit(1);
}
console.log(`i18n OK — ${en.size} keys present in both en and fa.`);
