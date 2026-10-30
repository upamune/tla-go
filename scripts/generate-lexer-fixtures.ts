// Run with Bun and a tla-ts checkout at the commit in README.md:
// bun scripts/generate-lexer-fixtures.ts /path/to/tla-ts > /tmp/lexer.json
// gzip -n -9 -c /tmp/lexer.json > syntax/testdata/upstream-lexer.json.gz
import { readdirSync, readFileSync } from "node:fs";
import { join, relative, resolve } from "node:path";

const upstream = resolve(process.argv[2]);
const { tokenize } = await import(join(upstream, "packages/tla-ts/src/lexer.ts"));
const root = join(upstream, "packages/tla-ts/tests/fixtures");
const fixtures: { name: string; source: string; tokens: unknown }[] = [];
function visit(dir: string) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) visit(path);
    else if (entry.name.endsWith(".tla")) {
      const source = readFileSync(path, "utf8");
      fixtures.push({ name: relative(root, path), source, tokens: tokenize(source) });
    }
  }
}
visit(root);
fixtures.sort((a, b) => a.name < b.name ? -1 : a.name > b.name ? 1 : 0);
process.stdout.write(JSON.stringify(fixtures));
