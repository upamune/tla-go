// bun scripts/generate-nested-string-order-matrix.ts /path/to/tla-ts > checker/testdata/upstream-nested-string-order-matrix.jsonl
import {cp, mkdtemp, readFile, writeFile} from 'node:fs/promises';
import {join, resolve} from 'node:path';
import {tmpdir} from 'node:os';

const dest = await mkdtemp(join(tmpdir(), 'tla-go-nested-string-order-'));
await cp(join(resolve(process.argv[2]), 'packages/tla-ts/src'), dest, {recursive: true});
const path = join(dest, 'tlc-order.ts');
const original = await readFile(path, 'utf8');
const patched = original.replace('const declare = (index: number, kind: SymbolKind)', 'const declareUnit = (index: number, kind: SymbolKind)').replaceAll('declare(index, "', 'declareUnit(index, "');
if (patched === original || patched.includes('declare(index,')) throw new Error('unexpected tlc-order source');
await writeFile(path, patched);
const {checkSequentially} = await import(join(dest, 'check.ts'));

for (let i = 0; i < 72; i++) {
  const items = ['xq', 'yq', 'zq'].map((_, j, names) => JSON.stringify(names[(j + i) % 3])).join(', ');
  const modules: Record<string, string> = {
    ExtA: `---- MODULE ExtA ----\nAValue == "${i % 2 ? 'yq' : 'zq'}"\n====`,
    ExtB: `---- MODULE ExtB ----\nBValue == "${i % 2 ? 'zq' : 'xq'}"\n====`,
  };
  const extend = i % 3 === 0 ? ', ExtA' : '';
  const top = i % 3 === 1 ? 'Top == INSTANCE ExtB\n' : '';
  const inner = i % 2 ? 'IB == INSTANCE ExtB\nIA == INSTANCE ExtA' : 'IA == INSTANCE ExtA\nIB == INSTANCE ExtB';
  const instance = i % 4 === 0 ? 'Nested == INSTANCE Inner\n' : '';
  const source = `---- MODULE M ----\nEXTENDS TLC${extend}\n${top}---- MODULE Inner ----\n${inner}\n====\n${instance}ASSUME PrintT({${items}})\n====`;
  const printed: string[] = [];
  const result = checkSequentially({source, config: '', loadModule: (name: string) => modules[name], onPrint: (s: string) => printed.push(s)});
  console.log(JSON.stringify({name: `nested-string-order-${i}`, source, modules, printed, status: result.status}));
}
