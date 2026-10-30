// bun scripts/generate-let-instance-matrix.ts /path/to/tla-ts > checker/testdata/upstream-let-instance-matrix.jsonl
import {cp, mkdtemp, readFile, writeFile} from 'node:fs/promises';
import {join, resolve} from 'node:path';
import {tmpdir} from 'node:os';

const root = resolve(process.argv[2]);
const dest = await mkdtemp(join(tmpdir(), 'tla-go-let-instance-'));
await cp(join(root, 'packages/tla-ts/src'), dest, {recursive: true});
const path = join(dest, 'tlc-order.ts');
const original = await readFile(path, 'utf8');
const patched = original.replace('const declare = (index: number, kind: SymbolKind)', 'const declareUnit = (index: number, kind: SymbolKind)').replaceAll('declare(index, "', 'declareUnit(index, "');
if (patched === original || patched.includes('declare(index,')) throw new Error('unexpected tlc-order source');
await writeFile(path, patched);
const {checkSequentially} = await import(join(dest, 'check.ts'));
const moduleSource = await readFile(join(root, 'packages/tla-ts/tests/fixtures/instances/Window.tla'), 'utf8');

for (let i = 0; i < 96; i++) {
  const width = i % 4 + 1;
  const bound = Math.floor(i / 4) % 8 + 1;
  const mode = Math.floor(i / 32);
  const move = mode === 2 ? `Move(n, v) == LET k == n\n I == INSTANCE Window WITH Width <- k, position <- v IN I!Advance\n` : '';
  const next = mode === 0 ? `LET k == ${width}\n I == INSTANCE Window WITH Width <- k, position <- x IN I!Advance` : mode === 1 ? `LET I(n) == INSTANCE Window WITH Width <- n, position <- x IN I(${width})!Advance` : `Move(${width}, x)`;
  const source = `---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\n${move}Init == x = 0\nNext == ${next}\nInv == x < ${bound}\n====`;
  const config = 'INIT Init NEXT Next INVARIANT Inv CHECK_DEADLOCK FALSE';
  const result = checkSequentially({source, config, loadModule: (name: string) => name === 'Window' ? moduleSource : undefined});
  console.log(JSON.stringify({name: `let-instance-${i}`, source, config, status: result.status, generated: result.generatedStates, distinct: result.distinctStates, depth: result.depth, trace: result.violation?.trace.map((step: any) => step.state.x) ?? null}));
}
