package domain

type Status string

const (
	StatusQueued     Status = "queued"
	StatusProcessing Status = "processing"
	StatusProcessed  Status = "processed"
)

type Verdict string

const (
	VerdictAllowed Verdict = "allowed"
	VerdictBlocked Verdict = "blocked"
)
