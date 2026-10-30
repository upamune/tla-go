// bun scripts/generate-state-view-multi-matrix.ts /path/to/tla-ts > checker/testdata/upstream-state-view-multi-matrix.jsonl
import {cp, mkdtemp, readFile, writeFile} from 'node:fs/promises';
import {join, resolve} from 'node:path';
import {tmpdir} from 'node:os';

const dest = await mkdtemp(join(tmpdir(), 'tla-go-state-view-multi-'));
await cp(join(resolve(process.argv[2]), 'packages/tla-ts/src'), dest, {recursive: true});
const path = join(dest, 'tlc-order.ts');
const original = await readFile(path, 'utf8');
const patched = original.replace('const declare = (index: number, kind: SymbolKind)', 'const declareUnit = (index: number, kind: SymbolKind)').replaceAll('declare(index, "', 'declareUnit(index, "');
if (patched === original || patched.includes('declare(index,')) throw new Error('unexpected tlc-order source');
await writeFile(path, patched);
const {checkSequentially} = await import(join(dest, 'check.ts'));

const formulas = [
  '<>(x=0 /\\ <>(x=1))',
  '[]<>(x=0) => []<>(x=1)',
  '<>[](x=0) \\/ []<>(x=1)',
  '<>(x=0 /\\ []<>(x=1))',
  '[](x=0 => <>(x=1)) /\\ []<>(x=2)',
  '[]<>(x=0) /\\ []<>(x=1)',
];
for (let graph = 0; graph < 16; graph++) {
  const edges = [0, 1, 2].map((node) => {
    const targets = [node, (node + 1) % 3, (node + 2) % 3].filter((_, i) => i === 0 || ((graph >> ((node + i) % 4)) & 1));
    return `/\\ x = ${node} /\\ x' \\in {${targets.join(',')}}`;
  });
  for (const fairness of ['']) {
    for (let i = 0; i < formulas.length; i++) {
      for (const view of ['x', 'y', 'x % 2', '0']) {
        const spec = fairness ? `Init /\\ [][Next]_vars /\\ ${fairness}` : 'Init /\\ [][Next]_vars';
        const source = `---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLES x, y\nvars == <<x,y>>\nInit == /\\ x = 0 /\\ y \\in {0,1}\nA == /\\ x' = (x+1)%3 /\\ y' = 1-y\nB == /\\ x' = (x+2)%3 /\\ y' = y\nStep(i) == x' = (x+i)%3\nA1 == \\/ x'=2 \\/ /\\ x'=1 /\\ 1 \\div 0=0\nA2 == \\/ x'=1 \\/ /\\ x'=2 /\\ 2 % 0=0\nNext == /\\ y' \\in {y,1-y} /\\ (\\/ ${edges.join('\n \\/ ')})\n${view ? `V == ${view}\n` : ''}Spec == ${spec}\nP == ${formulas[i]}\n====`;
        const config = `SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE${view ? ' VIEW V' : ''}`;
        const name = `state-view-multi-${graph}-${fairness || 'none'}-${i}-${view || 'no-view'}`;
        try {
          const printed: string[] = [];
          const r = checkSequentially({source, config, onPrint: (line: string) => printed.push(line)});
          console.log(JSON.stringify({name, source, config, printed, status: r.status, generated: r.generatedStates, distinct: r.distinctStates, depth: r.depth, queued: r.queuedStates, failure: r.failure, violation: r.violation, outDegrees: r.outDegrees}));
        } catch (error) {
          console.log(JSON.stringify({name, source, config, error: (error as Error).message}));
        }
      }
    }
  }
}
