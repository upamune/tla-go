// bun scripts/generate-tlc-string-order-data.ts /path/to/tla-ts > checker/tlc_string_order_data.go
import {join, resolve} from 'node:path';

const path = join(resolve(process.argv[2]), 'packages/tla-ts/src/tlc-modules.ts');
const data = await import(path);
console.log('// Code generated from tla-ts tlc-modules.ts at the pinned revision; DO NOT EDIT.\npackage checker\n');
for (const [name, source] of [
  ['tlcStartupStrings', data.TLC_STARTUP_STRINGS],
  ['sanyNodeNames', data.SANY_NODE_NAMES],
  ['tlcRuntimeStrings', data.TLC_RUNTIME_STRINGS],
]) {
  console.log(`var ${name} = []string{`);
  for (const text of source) console.log(`\t${JSON.stringify(text)},`);
  console.log('}\n');
}
