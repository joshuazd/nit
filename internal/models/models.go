package models

// LineType represents the type of a diff line.
type LineType int

const (
	LineContext    LineType = iota
	LineAdd
	LineRemove
	LineHunkHeader
)

func (lt LineType) String() string {
	switch lt {
	case LineContext:
		return "context"
	case LineAdd:
		return "add"
	case LineRemove:
		return "remove"
	case LineHunkHeader:
		return "hunk_header"
	default:
		return "unknown"
	}
}

// DiffLine represents a single line in a diff.
type DiffLine struct {
	Content   string
	LineType  LineType
	OldLineNo int // 0 = unset (line numbers start at 1)
	NewLineNo int // 0 = unset
	Raw       string
}

// DiffHunk represents a contiguous block of changes.
type DiffHunk struct {
	Header   string
	Lines    []DiffLine
	OldStart int
	NewStart int
}

// FileStatus represents the status of a file in a diff.
type FileStatus int

const (
	StatusModified FileStatus = iota
	StatusAdded
	StatusDeleted
	StatusRenamed
)

func (fs FileStatus) String() string {
	switch fs {
	case StatusModified:
		return "modified"
	case StatusAdded:
		return "added"
	case StatusDeleted:
		return "deleted"
	case StatusRenamed:
		return "renamed"
	default:
		return "unknown"
	}
}

// FileDiff represents the diff for a single file.
type FileDiff struct {
	Path     string
	OldPath  string
	Status   FileStatus
	Hunks    []DiffHunk
	IsBinary bool
}

// RowType represents the type of a side-by-side row.
type RowType int

const (
	RowContext    RowType = iota
	RowChange
	RowHunkHeader
)

// SideBySideRow pairs old/new lines for side-by-side display.
type SideBySideRow struct {
	Left    *DiffLine // nil = empty side
	Right   *DiffLine // nil = empty side
	RowType RowType
}

// ReviewComment is an inline review comment persisted to .nit.json.
type ReviewComment struct {
	FilePath    string   `json:"file"`
	NewLineNo   *int    `json:"line,omitempty"`
	OldLineNo   *int    `json:"old_line,omitempty"`
	LineContent string   `json:"line_content"`
	Comment     string   `json:"comment"`
	HunkContext []string `json:"hunk_context"`
	Timestamp   string   `json:"timestamp"`
	DiffMode    string   `json:"diff_mode"`
}

// IntPtr returns a pointer to an int value. Convenience for building ReviewComment.
func IntPtr(v int) *int {
	return &v
}
