package goal

import "github.com/SpherePrime/CLI/internal/session"

const (
	DefaultStallLimit = 3
	DefaultMaxSteps   = 200
)

type StopReason string

const (
	StopNone      StopReason = ""
	StopNoGoal    StopReason = "no_goal"
	StopAchieved  StopReason = "achieved"
	StopNoPlan    StopReason = "no_plan"
	StopStalled   StopReason = "stalled"
	StopStepLimit StopReason = "step_limit"
	StopQueued    StopReason = "queued"
)

type Driver struct {
	StallLimit int
	MaxSteps   int
	steps      int
	stalled    int
	last       string
	seen       bool
}

func NewDriver() *Driver {
	return &Driver{StallLimit: DefaultStallLimit, MaxSteps: DefaultMaxSteps}
}

func (d *Driver) Next(current Goal, found bool, todos []session.Todo) (string, StopReason) {
	if found && current.Status == StatusAchieved {
		return "", StopAchieved
	}
	if !found || !current.Active() {
		return "", StopNoGoal
	}
	if len(todos) == 0 {
		return "", StopNoPlan
	}
	mark := signature(todos)
	if d.seen && mark == d.last {
		d.stalled++
	} else {
		d.stalled = 0
	}
	d.last = mark
	d.seen = true
	if d.stalled >= d.StallLimit {
		return "", StopStalled
	}
	if d.steps >= d.MaxSteps {
		return "", StopStepLimit
	}
	d.steps++
	return ContinuationPrompt(ProgressOf(todos)), StopNone
}
