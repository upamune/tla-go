// bun scripts/benchmark-upstream-counter.ts /path/to/tla-ts
// Compare with: mise exec -- go test -run '^$' -bench=BenchmarkCheckCounter -benchmem -count=5 ./checker
import { join, resolve } from "node:path";
const root = join(resolve(process.argv[2]), "packages/tla-ts/src");
const { checkSequentially } = await import(join(root, "check.ts"));
const source = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == /\\ x < 1000 /\\ x' = x + 1\n====";
const config = "INIT Init NEXT Next CHECK_DEADLOCK FALSE";
for (let i = 0; i < 20; i++) checkSequentially({ source, config });
const times: number[] = [];
for (let run = 0; run < 5; run++) {
  const started = performance.now();
  for (let i = 0; i < 100; i++) {
    const result = checkSequentially({ source, config });
    if (result.status !== "success" || result.distinctStates !== 1001) throw new Error("incorrect result");
  }
  times.push((performance.now() - started) / 100);
}
console.log(`Milliseconds per check: ${times.join(", ")}`);
