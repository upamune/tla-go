// bun scripts/generate-parser-corpus.ts /path/to/tla-ts > /tmp/parser-corpus.jsonl 2> /tmp/parser-skipped.txt
// gzip -n -9 -c /tmp/parser-corpus.jsonl > syntax/parser/testdata/upstream-corpus.jsonl.gz
import { readdirSync, readFileSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { projectModule } from "./generate-parser-fixtures.ts";

const upstream = resolve(process.argv[2]);
const root = join(upstream, "packages/tla-ts/tests/fixtures");
const { parseModule } = await import(join(upstream, "packages/tla-ts/src/parser.ts"));
const files: string[] = [];
function visit(dir: string) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) visit(path);
    else if (entry.name.endsWith(".tla")) files.push(path);
  }
}
visit(root);
files.sort();
for (const file of files) {
  const name = relative(root, file);
  const source = readFileSync(file, "utf8");
  try {
    process.stdout.write(JSON.stringify({ name, source, want: projectModule(parseModule(source)) }) + "\n");
  } catch (error) {
    process.stderr.write(`${name}\t${(error as Error).message}\n`);
  }
}
