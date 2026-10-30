// bun scripts/generate-constant-view-ltl.ts /path/to/tla-ts > checker/testdata/upstream-constant-view-ltl.jsonl
import {cp, mkdtemp, readFile, writeFile} from 'node:fs/promises';
import {join, resolve} from 'node:path';
import {tmpdir} from 'node:os';

const dest = await mkdtemp(join(tmpdir(), 'tla-go-general-ltl-'));
await cp(join(resolve(process.argv[2]), 'packages/tla-ts/src'), dest, {recursive: true});
const path = join(dest, 'tlc-order.ts');
const original = await readFile(path, 'utf8');
const patched = original.replace('const declare = (index: number, kind: SymbolKind)', 'const declareUnit = (index: number, kind: SymbolKind)').replaceAll('declare(index, "', 'declareUnit(index, "');
if (patched === original || patched.includes('declare(index,')) throw new Error('unexpected tlc-order source');
await writeFile(path, patched);
const {checkSequentially} = await import(join(dest, 'check.ts'));

const formulas = [
  '[]<>[]TRUE',
  '[]<>[]FALSE',
  '<>[]<>TRUE',
  '<>[]<>FALSE',
  '([]<>TRUE) \\/ (<>[]FALSE)',
  '~([]<>TRUE /\\ <>[]FALSE)',
  '[]<>(IF TRUE THEN <>[]TRUE ELSE <>[]FALSE)',
  '<>(\\A i \\in {0,1} : []<>(i = i))',
];
for (let graph = 0; graph < 16; graph++) {
  const edges = [0, 1, 2].map((node) => {
    const targets = [node, (node + 1) % 3, (node + 2) % 3].filter((_, i) => i === 0 || ((graph >> ((node + i) % 4)) & 1));
    return `/\\ x = ${node} /\\ x' \\in {${targets.join(',')}}`;
  });
  for (const fairness of ['']) {
    for (let i = 0; i < formulas.length; i++) {
      for (const view of ['x % 2', '0']) {
        const spec = fairness ? `Init /\\ [][Next]_x /\\ ${fairness}` : 'Init /\\ [][Next]_x';
        const source = `---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == \\/ ${edges.join('\n \\/ ')}\n${view ? `V == ${view}\n` : ''}Spec == ${spec}\nP == ${formulas[i]}\n====`;
        const config = `SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE${view ? ' VIEW V' : ''}`;
        const name = `general-ltl-${graph}-${fairness || 'none'}-${i}-${view || 'no-view'}`;
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
