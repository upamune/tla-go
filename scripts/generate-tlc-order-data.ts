// bun scripts/generate-tlc-order-data.ts /path/to/tla-ts > checker/tlc_order_data.go
// Work around Bun stripping calls to a local function named `declare`.
import {cp, mkdtemp, readFile, writeFile} from 'node:fs/promises';
import {join,resolve} from 'node:path';
import {tmpdir} from 'node:os';
const root=resolve(process.argv[2]), dest=await mkdtemp(join(tmpdir(),'tla-go-tlc-order-'));
await cp(join(root,'packages/tla-ts/src'),dest,{recursive:true});
const path=join(dest,'tlc-order.ts'), original=await readFile(path,'utf8');
let patched=original.replace('const declare = (index: number, kind: SymbolKind)','const declareUnit = (index: number, kind: SymbolKind)').replaceAll('declare(index, "','declareUnit(index, "');
const expected=`return getContext(undefined)
    .enumerate()
    .filter((entry) => entry.kind === "variable")
    .map((entry) => entry.name)
    .reverse();`;
if(!patched.includes(expected) || patched.includes('declare(index,')) throw new Error('unexpected tlc-order source');
patched=patched.replace(expected,'return getContext(undefined).inserted as unknown as string[];');
await writeFile(path,patched);
const {getTlcVariableOrder}=await import(path);
const {TLC_MODULE_SOURCES,SANY_BUILTINS}=await import(join(dest,'tlc-modules.ts'));
const quote=(s:string)=>JSON.stringify(s);
console.log('// Code generated from tla-ts tlc-modules.ts at the pinned revision; DO NOT EDIT.\npackage checker\n');
console.log('var tlcBuiltinNames = []string{');
for(const name of SANY_BUILTINS) console.log(`\t${quote(name)},`);
console.log('}\n\nvar tlcStandardNames = map[string][]string{');
for(const name of ['Naturals','Integers','FiniteSets','Sequences','TLC','TLCExt','Bags','Json','Randomization']) {
 console.log(`\t${quote(name)}: {`);
 const source=TLC_MODULE_SOURCES.get(name);
 for(const entry of getTlcVariableOrder(source) as unknown as {name:string,isLocal:boolean}[]) {
  if(!entry.isLocal) console.log(`\t\t${quote(entry.name)},`);
 }
 console.log('\t},');
}
console.log('}');
