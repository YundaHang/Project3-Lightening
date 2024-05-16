package chainwriter

// Config is the ChainWriter's configuration options.
type Config struct {
	FileExtension    string
	DataDirectory    string
	BlockFileName    string
	UndoFileName     string
	MaxBlockFileSize uint32
	MaxUndoFileSize  uint32
}

// DefaultConfig returns the default Config for the ChainWriter.
func DefaultConfig() *Config {
	return &Config{
		FileExtension:    ".txt",
		DataDirectory:    "data",
		BlockFileName:    "block",
		UndoFileName:     "undo",
		MaxBlockFileSize: 1024,
		MaxUndoFileSize:  1024,
	}
}
