package git

// Status represents the outcome of a pull operation on a repository.
type Status string

const (
	StatusPending  Status = ""
	StatusOK       Status = "OK"
	StatusUpdated  Status = "UPDATED"
	StatusStashed  Status = "STASHED"
	StatusConflict Status = "CONFLICT"
	StatusSkipped  Status = "SKIPPED"
	StatusError    Status = "ERROR"
)

// Repo represents a discovered git repository.
type Repo struct {
	// Path is the absolute path to the repository root.
	Path string

	// RelPath is the path relative to the discovery root directory.
	RelPath string

	// Status is the outcome of the pull operation (populated by the runner).
	Status Status

	// Detail provides additional human-readable context about the status.
	Detail string
}
