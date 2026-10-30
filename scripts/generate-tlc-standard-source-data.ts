// bun scripts/generate-tlc-standard-source-data.ts /path/to/tla-ts > checker/tlc_standard_source_data.go
// Export upstream's private parsing helpers from a temporary copy.
import {cp, mkdtemp, readFile, writeFile} from 'node:fs/promises';
import {join, resolve} from 'node:path';
import {tmpdir} from 'node:os';

const dest = await mkdtemp(join(tmpdir(), 'tla-go-standard-sources-'));
await cp(join(resolve(process.argv[2]), 'packages/tla-ts/src'), dest, {recursive: true});
const path = join(dest, 'tlc-order.ts');
const original = await readFile(path, 'utf8');
let patched = original;
for (const name of ['getParsedTexts', 'getModuleReferences', 'getRanges']) {
  patched = patched.replace(`const ${name} =`, `export const ${name} =`);
}
if (patched === original) throw new Error('unexpected tlc-order source');
await writeFile(path, patched);
const {getParsedTexts, getModuleReferences, getRanges} = await import(path);
const {tokenize} = await import(join(dest, 'lexer.ts'));
const {TLC_MODULE_SOURCES} = await import(join(dest, 'tlc-modules.ts'));
const quote = (s: string) => JSON.stringify(s);
console.log('// Code generated from tla-ts standard modules at the pinned revision; DO NOT EDIT.\npackage checker\n');
console.log('import (\n "github.com/upamune/tla-go/syntax"\n "github.com/upamune/tla-go/syntax/parser"\n)');
console.log('\nvar tlcStandardSourceUnits = map[string]*parser.Module{');
for (const [name, source] of TLC_MODULE_SOURCES) {
  const tokens = tokenize(source);
  const range = getRanges(tokens).get(name);
  if (!range) throw new Error(`missing source range for ${name}`);
  const unit = tokens.slice(range.start, range.end);
  const refs = getModuleReferences(unit, name, new Map());
  if (refs.inner.length) throw new Error(`nested standard module in ${name}`);
  console.log(`${quote(name)}: {Name: ${quote(name)},`);
  console.log(`Extends: []string{${refs.extends.map(quote).join(',')}},`);
  console.log(`Instances: []parser.Instance{${refs.instances.map((n: string) => `{Module: ${quote(n)}}`).join(',')}},`);
  console.log('SourceTokens: []syntax.Token{');
  // Synthetic identifiers preserve parse-time texts. Appended literal tokens only
  // add already-seen quoted texts during parsing, and their contents during analysis.
  for (const text of getParsedTexts(unit)) console.log(`{Kind: "Identifier", Text: ${quote(text)}},`);
  for (const token of unit) if (token.kind === 'String') console.log(`{Kind: "String", Text: ${quote(token.text)}},`);
  console.log('}},');
}
console.log('}');
