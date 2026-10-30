// bun scripts/generate-lasso-action-matrix.ts /path/to/tla-ts > checker/testdata/upstream-lasso-action-matrix.jsonl
import {cp, mkdtemp, readFile, writeFile} from 'node:fs/promises';
import {join, resolve} from 'node:path';
import {tmpdir} from 'node:os';

const dest = await mkdtemp(join(tmpdir(), 'tla-go-lasso-action-'));
await cp(join(resolve(process.argv[2]), 'packages/tla-ts/src'), dest, {recursive: true});
const path = join(dest, 'tlc-order.ts');
const original = await readFile(path, 'utf8');
const patched = original.replace('const declare = (index: number, kind: SymbolKind)', 'const declareUnit = (index: number, kind: SymbolKind)').replaceAll('declare(index, "', 'declareUnit(index, "');
if (patched === original || patched.includes('declare(index,')) throw new Error('unexpected tlc-order source');
await writeFile(path, patched);
const {checkSequentially} = await import(join(dest, 'check.ts'));

const formulas = [
  "[]<><<A>>_x", "[]<><<B>>_x", "[]<>(x=1) => []<><<A>>_x",
  "[]<><<A>>_x => []<>(x=2)",
];
for (let graph = 0; graph < 16; graph++) {
  const edges = [0, 1, 2].map((node) => {
    const targets = [node, (node + 1) % 3, (node + 2) % 3].filter((_, i) => i === 0 || ((graph >> ((node + i) % 4)) & 1));
    return `/\\ x = ${node} /\\ x' \\in {${targets.join(',')}}`;
  });
  for (const fairness of ['WF_x(Next)', 'WF_x(Next) /\\ WF_x(A)']) {
    for (let i = 0; i < formulas.length; i++) {
      for (const view of ['', 'x']) {
        const spec = fairness ? `Init /\\ [][Next]_x /\\ ${fairness}` : 'Init /\\ [][Next]_x';
        const source = `---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0 /\\ PrintT("init") /\\ TLCSet(0,0)\nA == x' = (x+1)%3\nB == x' = (x+2)%3\nStep(i) == x' = (x+i)%3\nA1 == \\/ x'=2 \\/ /\\ x'=1 /\\ 1 \\div 0=0\nA2 == \\/ x'=1 \\/ /\\ x'=2 /\\ 2 % 0=0\nNext == /\\ PrintT(x) /\\ TLCSet(0, x+1) /\\ (\\/ ${edges.join('\n \\/ ')})\n${view ? `V == ${view}\n` : ''}Spec == ${spec}\nP == ${formulas[i]}\n====`;
        const config = `SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE${view ? ' VIEW V' : ''}`;
        const name = `lasso-action-${graph}-${fairness || 'none'}-${i}-${view || 'no-view'}`;
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
