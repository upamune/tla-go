// bun scripts/generate-fair-level-matrix.ts /path/to/tla-ts > checker/testdata/upstream-fair-level-matrix.jsonl
import {cp, mkdtemp, readFile, writeFile} from 'node:fs/promises';
import {join, resolve} from 'node:path';
import {tmpdir} from 'node:os';

const dest = await mkdtemp(join(tmpdir(), 'tla-go-fair-level-'));
await cp(join(resolve(process.argv[2]), 'packages/tla-ts/src'), dest, {recursive: true});
const path = join(dest, 'tlc-order.ts');
const original = await readFile(path, 'utf8');
const patched = original.replace('const declare = (index: number, kind: SymbolKind)', 'const declareUnit = (index: number, kind: SymbolKind)').replaceAll('declare(index, "', 'declareUnit(index, "');
if (patched === original || patched.includes('declare(index,')) throw new Error('unexpected tlc-order source');
await writeFile(path, patched);
const {checkSequentially} = await import(join(dest, 'check.ts'));

const formulas = [
  'WF_x(IF TLCGet("level") <= 1 THEN A ELSE B)',
  'SF_x(IF TLCGet("level") <= 2 THEN A ELSE B)',
  'WF_x(IF TLCGet("level") <= 2 THEN x=0 ELSE A)',
  'SF_x(IF TLCGet("level") <= 2 THEN x=1 ELSE B)',
  'WF_x(A) \\/ []<>(TLCGet("level")=2)',
  'SF_x(A) /\\ <>[](TLCGet("level")<=2)',
  'WF_x(A) => []<>(TLCGet("level")=1)',
  'WF_x(IF TLCGet("level")=2 THEN 1 \\div 0=0 ELSE A)',
  'SF_x(IF TLCGet("level")=2 THEN 2 % 0=0 ELSE A)',
  'WF_x(x=TLCGet("level"))',
  'WF_x(A) \\/ []<>(1 \\div (TLCGet("level")-1)=0)',
  'LET Level == TLCGet("level") IN SF_x(IF Level <= 2 THEN A ELSE B)',
];
for (let graph = 0; graph < 16; graph++) {
  const edges = [0, 1, 2].map((node) => {
    const targets = [node, (node + 1) % 3, (node + 2) % 3].filter((_, i) => i === 0 || ((graph >> ((node + i) % 4)) & 1));
    return `/\\ x = ${node} /\\ x' \\in {${targets.join(',')}}`;
  });
  for (const fairness of ['', 'WF_x(Next)', 'SF_x(Next)']) {
    for (let i = 0; i < formulas.length; i++) {
      for (const view of ['', 'x']) {
        const spec = fairness ? `Init /\\ [][Next]_x /\\ ${fairness}` : 'Init /\\ [][Next]_x';
        const source = `---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nA == x' = (x+1)%3\nB == x' = (x+2)%3\nStep(i) == x' = (x+i)%3\nA1 == \\/ x'=2 \\/ /\\ x'=1 /\\ 1 \\div 0=0\nA2 == \\/ x'=1 \\/ /\\ x'=2 /\\ 2 % 0=0\nNext == \\/ ${edges.join('\n \\/ ')}\n${view ? `V == ${view}\n` : ''}Spec == ${spec}\nP == ${formulas[i]}\n====`;
        const config = `SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE${view ? ' VIEW V' : ''}`;
        const name = `fair-level-${graph}-${fairness || 'none'}-${i}-${view || 'no-view'}`;
        try {
          const r = checkSequentially({source, config});
          console.log(JSON.stringify({name, source, config, status: r.status, generated: r.generatedStates, distinct: r.distinctStates, depth: r.depth, queued: r.queuedStates, failure: r.failure, violation: r.violation, outDegrees: r.outDegrees}));
        } catch (error) {
          console.log(JSON.stringify({name, source, config, error: (error as Error).message}));
        }
      }
    }
  }
}
