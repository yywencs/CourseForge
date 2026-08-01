package catalog

import (
	"time"

	domain "prizeforge/internal/domain/catalog"
)

type SelectionRoundCommand struct {
	TermID    uint64
	RoundCode string
	RoundName string
	StartTime time.Time
	EndTime   time.Time
}

func (c SelectionRoundCommand) plan() domain.SelectionRoundPlan {
	return domain.SelectionRoundPlan{
		TermID: c.TermID, RoundCode: c.RoundCode, RoundName: c.RoundName,
		StartTime: c.StartTime, EndTime: c.EndTime,
	}
}
