// bun scripts/generate-fair-property-matrix.ts /path/to/tla-ts > checker/testdata/upstream-fair-property-matrix.jsonl
import {cp, mkdtemp, readFile, writeFile} from 'node:fs/promises';
import {join, resolve} from 'node:path';
import {tmpdir} from 'node:os';

const dest = await mkdtemp(join(tmpdir(), 'tla-go-fair-property-'));
await cp(join(resolve(process.argv[2]), 'packages/tla-ts/src'), dest, {recursive: true});
const path = join(dest, 'tlc-order.ts');
const original = await readFile(path, 'utf8');
const patched = original.replace('const declare = (index: number, kind: SymbolKind)', 'const declareUnit = (index: number, kind: SymbolKind)').replaceAll('declare(index, "', 'declareUnit(index, "');
if (patched === original || patched.includes('declare(index,')) throw new Error('unexpected tlc-order source');
await writeFile(path, patched);
const {checkSequentially} = await import(join(dest, 'check.ts'));

const formulas = [
  'WF_x(A)',
  'SF_x(A)',
  '~WF_x(A)',
  '~SF_x(A)',
  'WF_x(A) \\/ []<>(x=0)',
  'SF_x(A) /\\ []<>(x=1)',
  'WF_x(A) => []<>(x=1)',
  'SF_x(A) => <>[](x=2)',
  '[](x=0 => WF_x(A))',
  'LET Move == A IN WF_x(Move) \\/ []<>(x=0)',
  '\\A i \\in {0,1} : (WF_x(Step(i)) \\/ []<>(x=i))',
  'WF_x(A) \\/ SF_x(B)',
  '[]WF_x(A)',
  '<>WF_x(A)',
  '[]SF_x(A)',
  '<>SF_x(A)',
  'WF_x(A) => SF_x(B)',
  'SF_x(A) => WF_x(B)',
  '~(WF_x(A) /\\ SF_x(B))',
  '(WF_x(A) \\/ SF_x(B)) /\\ []<>(x=0)',
  '[]<>(WF_x(A) \\/ x=0)',
  '<>[]WF_x(A)',
  'IF x=0 THEN WF_x(A) ELSE SF_x(B)',
  'LET R == WF_x(A) IN ~R \\/ []<>(x=1)',

  'LET Move(i) == x\'=(x+i)%3 IN \\E i \\in {1,2} : SF_x(Move(i))',
  'WF_<<x>>(A)',
  'WF_x(UNCHANGED x)',
  'WF_x(x=0)',
  'SF_x(A /\\ x=2)',
  'WF_x(x\' \\in {0,1})',
  '[]<>(WF_x(UNCHANGED x))',
  'LET Move(i) == x\'=(x+i)%3 IN WF_x(Move(1)) => <>[](x=0)',

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
        const source = `---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nA == x' = (x+1)%3\nB == x' = (x+2)%3\nStep(i) == x' = (x+i)%3\nNext == \\/ ${edges.join('\n \\/ ')}\n${view ? `V == ${view}\n` : ''}Spec == ${spec}\nP == ${formulas[i]}\n====`;
        const config = `SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE${view ? ' VIEW V' : ''}`;
        const name = `fair-property-${graph}-${fairness || 'none'}-${i}-${view || 'no-view'}`;
        try {
          const r = checkSequentially({source, config});
          console.log(JSON.stringify({name, source, config, status: r.status, generated: r.generatedStates, distinct: r.distinctStates, depth: r.depth}));
        } catch (error) {
          console.log(JSON.stringify({name, source, config, error: (error as Error).message}));
        }
      }
    }
  }
}
