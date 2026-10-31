These files drive `TestJavaPlusCalCorpus`, which compares tla-go's PlusCal
translator with `pcal.trans` from the Java TLA+ tools 1.7.4.

`upstream/` holds unchanged copies of the 106 `.tla` files of the TLA+ tools
repository at commit `5a47802b5c391f59ecdd44117981f4ff8c0656ba` that contain
`--algorithm` or `--fair algorithm`, together with their `.cfg` files. The
paths below `upstream/` are the paths in that repository. Most come from
`tlatools/org.lamport.tlatools/test-model/pcal`, which the `pcal` JUnit
tests translate before model checking. Some files mention an algorithm
without containing a valid one; they exercise error reporting.

`junit/` holds the inline PlusCal sources of the `AssignmentToUndeclaredVariableTest`,
`DivergenceTest`, `OptionalSemicolonTest`, and `UnhandledInvalidSyntaxTest`
JUnit tests in `tlatools/org.lamport.tlatools/test/pcal` at the same commit.
Each file is named after its test class and method. The string literals were
concatenated as Java does, and the dynamic module names of `DivergenceTest`
and `OptionalSemicolonTest` were replaced by the file name.

`extra/` holds inputs written for tla-go. They cover options, error messages,
non-ASCII text, tabs, CRLF line ends, and existing `.cfg` files.

`mutations/` holds copies of upstream files with one or two tokens of the
algorithm deleted, inserted, or replaced at random, keeping one input for
each distinct Java error message found. The `source` field in `cases.json`
names the file each one was derived from.

`cases.json` lists each run: the input directory and files, empty
directories to create in place of unreadable files, the arguments of
`pcal.trans`, and whether to remove the existing translation first, so
that the translator inserts a new one with checksums. The JUnit models run
with the options their tests use and with several other option sets.

`golden.json.gz` holds Java's exit code, standard output, and resulting
files for each case. Files equal to an input file are recorded by name.
Java ran with `LC_ALL=C.UTF-8`, because it encodes its messages in the
locale's charset; tla-go always writes UTF-8. To compare with Java directly
or to regenerate the file, run from the repository root:

```sh
TLC_JAR=/path/to/tla2tools.jar go test ./pcal -run TestJavaPlusCalCorpus
TLC_JAR=/path/to/tla2tools.jar PCAL_UPDATE_GOLDEN=1 go test ./pcal -run TestJavaPlusCalCorpus
```

The TLA+ tools are distributed under the MIT license; see `LICENSE` in this
directory.
