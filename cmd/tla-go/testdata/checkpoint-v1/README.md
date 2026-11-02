# Version 1 checkpoint fixture

`tla-go.chkpt` was written by a `tla-go` executable built from commit
`96ebfc0`, with the `M.tla` and `M.cfg` in this directory. It was paused after
expanding two states using:

```sh
TLA_GO_DEBUG_CHECKPOINT_AFTER=2 tla-go -tool -workers 1 -fp 0 \
  -checkpoint 1 -metadir states M.tla
```

The binary checkpoint was copied from the generated metadata directory. Its
source, configuration, and semantic options must stay unchanged for recovery.
