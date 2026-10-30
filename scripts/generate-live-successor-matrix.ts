// bun scripts/generate-live-successor-matrix.ts /path/to/tla-ts > checker/testdata/upstream-live-successor-matrix.jsonl
import {cp, mkdtemp, readFile, writeFile} from 'node:fs/promises';
import {join, resolve} from 'node:path';
import {tmpdir} from 'node:os';

const dest = await mkdtemp(join(tmpdir(), 'tla-go-fair-failure-'));
await cp(join(resolve(process.argv[2]), 'packages/tla-ts/src'), dest, {recursive: true});
const path = join(dest, 'tlc-order.ts');
const original = await readFile(path, 'utf8');
const patched = original.replace('const declare = (index: number, kind: SymbolKind)', 'const declareUnit = (index: number, kind: SymbolKind)').replaceAll('declare(index, "', 'declareUnit(index, "');
if (patched === original || patched.includes('declare(index,')) throw new Error('unexpected tlc-order source');
await writeFile(path, patched);
const {checkSequentially} = await import(join(dest, 'check.ts'));

for (const size of [3, 7, 8, 9, 16, 24]) {
  for (let special = 1; special <= size; special++) {
    for (const fair of ['WF', 'SF']) {
      for (const view of ['', ' VIEW V']) {
        const source = `---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x=0\nNext == x' \\in 0..${size}\nA == \\/ x'=99 \\/ /\\ x' \\in 1..${size} /\\ IF x'=${special} THEN 1 \\div 0=0 ELSE 2 % 0=0\nV == x\nP == ${fair}_x(A)\n====`;
        const config = `INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE${view}`;
        const name = `live-successor-${size}-${special}-${fair}-${view ? 'view' : 'raw'}`;
        try {
          const r = checkSequentially({source, config});
          console.log(JSON.stringify({name, source, config, status: r.status, generated: r.generatedStates, distinct: r.distinctStates, depth: r.depth, queued: r.queuedStates, failure: r.failure, outDegrees: r.outDegrees}));
        } catch (error) {
          console.log(JSON.stringify({name, source, config, error: (error as Error).message}));
        }
      }
    }
  }
}
