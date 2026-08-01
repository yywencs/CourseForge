package roundrepo

import (
	"context"
	"time"

	enrollmentapp "prizeforge/internal/enrollment/application"
	"prizeforge/internal/enrollment/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type roundRow struct {
	ID         uint64    `gorm:"column:id;primaryKey"`
	TermID     uint64    `gorm:"column:term_id"`
	RoundCode  string    `gorm:"column:round_code"`
	RoundName  string    `gorm:"column:round_name"`
	StartTime  time.Time `gorm:"column:start_time"`
	EndTime    time.Time `gorm:"column:end_time"`
	State      string    `gorm:"column:state"`
	ClassCount int64     `gorm:"column:class_count;->"`
	CreateTime time.Time `gorm:"column:create_time"`
	UpdateTime time.Time `gorm:"column:update_time"`
}

func (roundRow) TableName() string { return "selection_round" }

func (r roundRow) aggregate() enrollment.SelectionRound {
	return enrollment.SelectionRound{
		ID: r.ID, TermID: r.TermID, RoundCode: r.RoundCode, RoundName: r.RoundName,
		StartTime: r.StartTime, EndTime: r.EndTime, State: enrollment.SelectionRoundState(r.State),
		CreateTime: r.CreateTime, UpdateTime: r.UpdateTime,
	}
}

func (r roundRow) view() enrollmentapp.SelectionRoundView {
	return enrollmentapp.SelectionRoundView{
		ID: r.ID, TermID: r.TermID, RoundCode: r.RoundCode, RoundName: r.RoundName,
		StartTime: r.StartTime, EndTime: r.EndTime, State: enrollment.SelectionRoundState(r.State),
		ClassCount: r.ClassCount, CreateTime: r.CreateTime, UpdateTime: r.UpdateTime,
	}
}

type bindingRow struct {
	ID              uint64    `gorm:"column:id;primaryKey"`
	RoundID         uint64    `gorm:"column:round_id"`
	TeachingClassID uint64    `gorm:"column:teaching_class_id"`
	ClassCode       string    `gorm:"column:class_code;->"`
	CourseName      string    `gorm:"column:course_name;->"`
	State           string    `gorm:"column:state"`
	CreateTime      time.Time `gorm:"column:create_time"`
}

func (bindingRow) TableName() string { return "selection_round_class" }

func (r *Repository) ListRounds(ctx context.Context, termID uint64) ([]enrollmentapp.SelectionRoundView, error) {
	db := r.dbFor(ctx).Table("selection_round AS sr").
		Select(`sr.id, sr.term_id, sr.round_code, sr.round_name, sr.start_time,
			sr.end_time, sr.state, sr.create_time, sr.update_time,
			COUNT(src.id) AS class_count`).
		Joins("LEFT JOIN selection_round_class AS src ON src.round_id = sr.id").
		Group("sr.id")
	if termID > 0 {
		db = db.Where("sr.term_id = ?", termID)
	}
	var rows []roundRow
	if err := db.Order("sr.term_id DESC, sr.start_time DESC, sr.id DESC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]enrollmentapp.SelectionRoundView, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.view())
	}
	return items, nil
}

func (r *Repository) GetRoundForUpdate(ctx context.Context, id uint64) (*enrollment.SelectionRound, error) {
	var row roundRow
	err := r.dbFor(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Take(&row, "id = ?", id).Error
	if err != nil {
		return nil, normalizeDBError(err)
	}
	round := row.aggregate()
	return &round, nil
}

func (r *Repository) InsertRound(ctx context.Context, round *enrollment.SelectionRound) error {
	row := roundRow{
		TermID: round.TermID, RoundCode: round.RoundCode, RoundName: round.RoundName,
		StartTime: round.StartTime, EndTime: round.EndTime, State: string(round.State),
	}
	if err := r.dbFor(ctx).Create(&row).Error; err != nil {
		return normalizeDBError(err)
	}
	*round = row.aggregate()
	return nil
}

func (r *Repository) SaveRound(ctx context.Context, round *enrollment.SelectionRound) error {
	result := r.dbFor(ctx).Model(&roundRow{}).
		Where("id = ? AND state = ?", round.ID, string(enrollment.SelectionRoundStatePlanned)).
		Updates(map[string]interface{}{
			"term_id": round.TermID, "round_code": round.RoundCode, "round_name": round.RoundName,
			"start_time": round.StartTime, "end_time": round.EndTime,
			"update_time": gorm.Expr("GREATEST(DATE_ADD(update_time, INTERVAL 1 MILLISECOND), NOW(3))"),
		})
	return requireConditionalWrite(result)
}

func (r *Repository) RemoveRound(ctx context.Context, id uint64) error {
	result := r.dbFor(ctx).Delete(&roundRow{}, "id = ? AND state = ?", id, string(enrollment.SelectionRoundStatePlanned))
	return requireConditionalWrite(result)
}

func (r *Repository) InspectRoundUsage(ctx context.Context, id uint64) (enrollment.SelectionRoundUsage, error) {
	db := r.dbFor(ctx)
	var usage enrollment.SelectionRoundUsage
	queries := []struct {
		table  string
		target *int64
	}{
		{table: "selection_round_class", target: &usage.ClassBindingCount},
		{table: "student_selection_quota", target: &usage.QuotaCount},
		{table: "selection_application", target: &usage.ApplicationCount},
		{table: "selection_waitlist", target: &usage.WaitlistCount},
	}
	for _, query := range queries {
		if err := db.Table(query.table).Where("round_id = ?", id).Count(query.target).Error; err != nil {
			return enrollment.SelectionRoundUsage{}, err
		}
	}
	return usage, nil
}

func (r *Repository) GetRoundClassCandidateForUpdate(
	ctx context.Context,
	teachingClassID uint64,
) (enrollment.RoundClassCandidate, error) {
	var row struct {
		TermID uint64
		State  string
	}
	err := r.dbFor(ctx).Table("teaching_class").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("term_id, state").
		Where("id = ?", teachingClassID).
		Take(&row).Error
	if err != nil {
		return enrollment.RoundClassCandidate{}, normalizeDBError(err)
	}
	return enrollment.RoundClassCandidate{
		TermID: row.TermID,
		State:  enrollment.BindingTeachingClassState(row.State),
	}, nil
}

func (r *Repository) ListRoundClasses(ctx context.Context, roundID uint64) ([]enrollmentapp.RoundClassBindingView, error) {
	var rows []bindingRow
	err := r.dbFor(ctx).Table("selection_round_class AS src").
		Select("src.id, src.round_id, src.teaching_class_id, src.state, src.create_time, tc.class_code, c.course_name").
		Joins("JOIN teaching_class AS tc ON tc.id = src.teaching_class_id").
		Joins("JOIN course AS c ON c.id = tc.course_id").
		Where("src.round_id = ?", roundID).
		Order("tc.class_code ASC, src.id ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	items := make([]enrollmentapp.RoundClassBindingView, 0, len(rows))
	for _, row := range rows {
		items = append(items, enrollmentapp.RoundClassBindingView{
			ID: row.ID, RoundID: row.RoundID, TeachingClassID: row.TeachingClassID,
			ClassCode: row.ClassCode, CourseName: row.CourseName, State: row.State,
			CreateTime: row.CreateTime,
		})
	}
	return items, nil
}

func (r *Repository) InsertRoundClass(ctx context.Context, roundID, teachingClassID uint64) error {
	return normalizeDBError(r.dbFor(ctx).Create(&bindingRow{
		RoundID: roundID, TeachingClassID: teachingClassID, State: "open",
	}).Error)
}

func (r *Repository) RemoveRoundClass(ctx context.Context, roundID, teachingClassID uint64) error {
	result := r.dbFor(ctx).Where("round_id = ? AND teaching_class_id = ?", roundID, teachingClassID).Delete(&bindingRow{})
	if result.Error != nil {
		return normalizeDBError(result.Error)
	}
	if result.RowsAffected != 1 {
		return enrollment.ErrNotFound
	}
	return nil
}
