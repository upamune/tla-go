// bun scripts/generate-standard-string-order-matrix.ts /path/to/tla-ts > checker/testdata/upstream-standard-string-order-matrix.jsonl
// Bun strips calls to a local function named `declare`; rename it in a temporary copy.
import {cp, mkdtemp, readFile, writeFile} from 'node:fs/promises';
import {join, resolve} from 'node:path';
import {tmpdir} from 'node:os';

const dest = await mkdtemp(join(tmpdir(), 'tla-go-standard-string-order-'));
await cp(join(resolve(process.argv[2]), 'packages/tla-ts/src'), dest, {recursive: true});
const path = join(dest, 'tlc-order.ts');
const original = await readFile(path, 'utf8');
const patched = original.replace('const declare = (index: number, kind: SymbolKind)', 'const declareUnit = (index: number, kind: SymbolKind)').replaceAll('declare(index, "', 'declareUnit(index, "');
if (patched === original || patched.includes('declare(index,')) throw new Error('unexpected tlc-order source');
await writeFile(path, patched);
const {checkSequentially} = await import(join(dest, 'check.ts'));

const standards = ['Naturals', 'Integers', 'Sequences', 'FiniteSets', 'TLC', 'TLCExt', 'Bags'];
const names = ['Nat', 'Int', 'Len', 'Cardinality', 'Print', 'Append', 'AssertEq', 'BagUnion', 'xq', 'yq', 'TRUE', 'FALSE'];
for (let i = 0; i < 196; i++) {
  const standard = standards[i % standards.length];
  const shifted = names.map((_, j) => names[(j * 5 + i) % names.length]);
  const items = shifted.map((s) => JSON.stringify(s)).join(', ');
  const useInstance = i % 4 === 3;
  const extra = i % 4 === 1 ? 'User, ' : '';
  const rootModules = [...new Set([...(extra ? ['User'] : []), standard, 'TLC', ...(useInstance ? ['Naturals'] : [])])].join(', ');
  const modules: Record<string, string> = {
    User: `---- MODULE User ----\nEXTENDS Naturals\nUserFirst == "${i % 2 ? 'yq' : 'xq'}"\nUserSecond == "${i % 2 ? 'xq' : 'yq'}"\n====`,
  };
  const source = `---- MODULE M ----\nEXTENDS ${rootModules}\nCONSTANT P\n${useInstance ? 'I == INSTANCE User\n' : ''}ASSUME PrintT({${items}})\nASSUME PrintT(P)\n====`;
  const config = i % 3 === 0 ? 'CONSTANT P = {"xq", "Nat", "yq"}' : 'CONSTANT P = {"yq", "Int", "xq"}';
  const printed: string[] = [];
  const result = checkSequentially({source, config, loadModule: (name: string) => modules[name], onPrint: (s: string) => printed.push(s)});
  console.log(JSON.stringify({name: `standard-string-order-${i}`, source, config, modules, printed, status: result.status}));
}
