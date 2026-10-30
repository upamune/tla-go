// bun scripts/generate-let-ltl-matrix.ts /path/to/tla-ts > checker/testdata/upstream-let-ltl-matrix.jsonl
import {cp, mkdtemp, readFile, writeFile} from 'node:fs/promises';
import {join, resolve} from 'node:path';
import {tmpdir} from 'node:os';

const dest = await mkdtemp(join(tmpdir(), 'tla-go-let-ltl-'));
await cp(join(resolve(process.argv[2]), 'packages/tla-ts/src'), dest, {recursive: true});
const path = join(dest, 'tlc-order.ts');
const original = await readFile(path, 'utf8');
const patched = original.replace('const declare = (index: number, kind: SymbolKind)', 'const declareUnit = (index: number, kind: SymbolKind)').replaceAll('declare(index, "', 'declareUnit(index, "');
if (patched === original || patched.includes('declare(index,')) throw new Error('unexpected tlc-order source');
await writeFile(path, patched);
const {checkSequentially} = await import(join(dest, 'check.ts'));

const formulas = [
  'LET A == <>[](x=0) IN []<>A',
  'LET A == x=0 B == <>[]A IN []<>B',
  'LET A(y) == <>[](x=y) IN []<>A(0)',
  'LET A == x=0 B == <>[]A IN LET A == x=1 IN []<>B',
  'LET A == x=0 F(y) == <>[](y) IN LET A == x=1 IN []<>F(A)',
  'LET S == {0,1} F(y) == <>[](x=y) IN []<>(\\A y \\in S : F(y))',
  'LET A == TRUE IN []<>(IF A THEN <>[](x=0) ELSE <>[](x=1))',
  'LET A == <<x\' = (x+1)%3>>_x IN []<>A \\/ []<>(x=0)',
  'LET A == <>[](x=0) IN (A /\\ []<>(x=1))',
  'LET A == Z Z == x=1 IN []<>[]A',
  'LET A == <>[](x=0) IN LET B == A IN []<>B',
  'LET A == <>[](x=0) IN ~(A \\/ []<>(x=1))',
  'LET F(y) == []<>(x=y) IN F(0) \\/ F(1)',
  'LET F(y) == <>[](x=y) G(y) == []<>F(y) IN G(0) /\\ G(1)',
  'LET K == 1 A == x=K IN []<>[]A',
  'LET S == {0,1} IN []<>(\\E y \\in S : <>[](x=y))',
  'LET F(y) == x=y IN []<>[](F(0) \\/ F(1))',
  'LET C == x=0 IN []<>(IF C THEN <>[](x=1) ELSE []<>(x=2))',
  '[]<>(LET A == x=0 IN <>[]A)',
  'LET A == x=0 IN []<>(LET B == x=1 IN <>[](A \\/ B))',
  'LET A == x=0 B(y) == <>[](y) IN []<>B(A)',
  'LET A == x=0 IN []<><<x\'=x+1>>_x \\/ []<>A',
  'LET S == {0,1} IN \\A y \\in S : (<>[](x=y) \\/ []<>(x=2))',
  'LET F(y) == <>[](x=y) IN \\A y \\in {0,1} : []<>F(y)',
  'LET F(y) == x=0 IN []<>[]F(1 \\div 0)',
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
        const source = `---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nZ == x = 2\nNext == \\/ ${edges.join('\n \\/ ')}\n${view ? `V == ${view}\n` : ''}Spec == ${spec}\nP == ${formulas[i]}\n====`;
        const config = `SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE${view ? ' VIEW V' : ''}`;
        const name = `let-ltl-${graph}-${fairness || 'none'}-${i}-${view || 'no-view'}`;
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
