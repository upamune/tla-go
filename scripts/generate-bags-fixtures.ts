// bun scripts/generate-bags-fixtures.ts /path/to/tla-ts > evaluator/testdata/upstream-bags.jsonl
import {join,resolve} from 'node:path';
const root=join(resolve(process.argv[2]),'packages/tla-ts/src');
const {createEnvironment}=await import(join(root,'environment.ts'));
const {parseConfig}=await import(join(root,'config.ts'));
const {evaluate}=await import(join(root,'evaluate.ts'));
const {formatValue}=await import(join(root,'values.ts'));
const exprs=[
 'EmptyBag','IsABag(<<>>)','IsABag(<<1,2,3>>)','IsABag(<<0>>)','IsABag({1})',
 'BagCardinality(<<1,2,3>>)','BagIn(2, <<1,0,3>>)','BagIn(4, <<1,2,3>>)','CopiesIn(2, <<1,-1>>)','CopiesIn(4, <<1,2>>)',
 'BagToSet(<<1,0,3>>)','SetToBag({"a", "b"})','<<1,2>> (+) <<2,3>>','<<1,2>> \\oplus <<2,3>>',
 '<<2,4,1>> (-) <<1,2,1>>','<<2,4,1>> \\ominus <<1,2,1>>','<<0,-1,2>> (-) EmptyBag',
 '<<1,2>> \\sqsubseteq <<2,3>>','<<2,3>> \\sqsubseteq <<1,2>>','<<0>> \\sqsubseteq EmptyBag',
 'BagUnion({})','BagUnion({1})','BagUnion({<<1,2>>, <<2,1>>})','BagUnion({<<>>, <<1,1,1>>, <<1,2,3,4>>})',
 'SubBag(EmptyBag)','SubBag(<<2,1>>)','Cardinality(SubBag(<<2,1,3>>))','BagOfAll(LAMBDA n : n % 2, <<1,2,3,4>>)',
 'BagOfAll(LAMBDA e : TRUE, <<1,2,3>>)','LET Inc(n) == n + 1 IN BagOfAll(Inc, <<1,2>>)',
 'LET Twice(F(_), b) == BagOfAll(F, b) IN Twice(LAMBDA n : n % 2, <<1,2,3>>)',
 'LET F(B(_, _), a, b) == B(a, b) IN F(\\oplus, <<1>>, <<2>>)',
];
for(const expr of exprs){
 const source=`---- MODULE M ----\nEXTENDS Bags, Integers, FiniteSets, TLC\nResult == ${expr}\n====`;
 try {
  const {environment}=createEnvironment(parseConfig(''),{source,loadModule:undefined,print:()=>{},files:{readFile:undefined,writeFile:undefined}});
  const result=evaluate(environment.definitions.get('Result').body,{environment,scope:undefined,state:[],next:[]});
  console.log(JSON.stringify({expr,want:formatValue(result)}));
 } catch(e){throw new Error(expr+': '+(e as Error).message)}
}
