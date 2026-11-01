package checker

// Windows does not support flushing directory handles through os.File.Sync.
// The replacement file itself has already been synced before the rename.
func checkpointSyncDir(_ string) error { return nil }
