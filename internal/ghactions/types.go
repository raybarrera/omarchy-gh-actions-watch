package ghactions

// Run is a single active GitHub Actions workflow run, normalized for display.
type Run struct {
	Repo         string `json:"repo"`
	Workflow     string `json:"workflow"`
	WorkflowID   int64  `json:"workflowId"`
	RunID        int64  `json:"runId"`
	RunNumber    int    `json:"runNumber"`
	Branch       string `json:"branch"`
	Event        string `json:"event"`
	Status       string `json:"status"`
	Conclusion   string `json:"conclusion"`
	Actor        string `json:"actor"`
	DisplayTitle string `json:"displayTitle"`
	CreatedAt    string `json:"createdAt"`
	StartedAt    string `json:"startedAt"`
	UpdatedAt    string `json:"updatedAt"`
	URL          string `json:"url"`
	ElapsedSec   int64  `json:"elapsedSec"`
}

// Snapshot is the JSON document the binary writes to stdout for the shell to read.
type Snapshot struct {
	Account       string `json:"account"`
	Host          string `json:"host"`
	GeneratedAt   string `json:"generatedAt"`
	Running       []Run  `json:"running"`
	Queued        []Run  `json:"queued"`
	RunningCount  int    `json:"runningCount"`
	QueuedCount   int    `json:"queuedCount"`
	ActiveCount   int    `json:"activeCount"`
	ReposScanned  int    `json:"reposScanned"`
	DurationSec   int64  `json:"durationSec"`
	APICalls      int    `json:"apiCalls"`
	RateRemaining int    `json:"rateRemaining"`
	RateLimited   bool   `json:"rateLimited"`
	Stale         bool   `json:"stale"`
	PausedUntil   string `json:"pausedUntil,omitempty"`
	Error         string `json:"error,omitempty"`
}

// Options configure a single snapshot run.
type Options struct {
	Account       string
	Host          string
	MaxRepos      int
	IncludeQueued bool
}

type actor struct {
	Login string `json:"login"`
}

type apiRun struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	WorkflowID   int64   `json:"workflow_id"`
	RunNumber    int     `json:"run_number"`
	Status       string  `json:"status"`
	Conclusion   *string `json:"conclusion"`
	HeadBranch   string  `json:"head_branch"`
	Event        string  `json:"event"`
	DisplayTitle string  `json:"display_title"`
	HTMLURL      string  `json:"html_url"`
	CreatedAt    string  `json:"created_at"`
	RunStartedAt *string `json:"run_started_at"`
	UpdatedAt    string  `json:"updated_at"`
	Actor        *actor  `json:"actor"`
}

type runsPage struct {
	TotalCount   int      `json:"total_count"`
	WorkflowRuns []apiRun `json:"workflow_runs"`
}

type apiRepo struct {
	FullName string `json:"full_name"`
	PushedAt string `json:"pushed_at"`
	Private  bool   `json:"private"`
}

type apiUser struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}
