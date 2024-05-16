package chainwriter

// FileInfo determines where a Block or UndoBlock is stored.
type FileInfo struct {
	FileName    string
	StartOffset uint32
	EndOffset   uint32
}
