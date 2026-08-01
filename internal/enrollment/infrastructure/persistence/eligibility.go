package enrollmentrepo

import (
	"context"
	"database/sql"
	"errors"

	"prizeforge/internal/enrollment/domain"

	"gorm.io/gorm"
)

type scheduleRow struct {
	DayOfWeek    uint8
	StartWeek    uint8
	EndWeek      uint8
	StartSection uint8
	EndSection   uint8
}

func (r *EligibilityStore) QueryEligibilitySnapshot(
	ctx context.Context,
	studentID uint64,
	termID uint64,
	courseID uint64,
	teachingClassID uint64,
) (*enrollment.EligibilitySnapshot, error) {
	if studentID == 0 || termID == 0 || courseID == 0 || teachingClassID == 0 {
		return nil, enrollment.ErrInvalidParams
	}

	snapshot := &enrollment.EligibilitySnapshot{}
	var studentRow struct {
		ID        uint64
		MajorID   uint64
		GradeYear uint16
		State     string
	}
	err := r.db.WithContext(ctx).
		Table("student").
		Select("id, major_id, grade_year, state").
		Where("id = ?", studentID).
		Take(&studentRow).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, enrollment.ErrRecordNotFound
	}
	if err != nil {
		return nil, err
	}
	snapshot.Student = &enrollment.StudentProfile{
		ID:        studentRow.ID,
		MajorID:   studentRow.MajorID,
		GradeYear: studentRow.GradeYear,
		State:     enrollment.StudentState(studentRow.State),
	}

	var classRow struct {
		MinimumGradeYear sql.NullInt64
		MaximumGradeYear sql.NullInt64
	}
	err = r.db.WithContext(ctx).
		Table("teaching_class").
		Select("minimum_grade_year, maximum_grade_year").
		Where("id = ? AND term_id = ? AND course_id = ?", teachingClassID, termID, courseID).
		Take(&classRow).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, enrollment.ErrRecordNotFound
	}
	if err != nil {
		return nil, err
	}
	if classRow.MinimumGradeYear.Valid {
		value := uint16(classRow.MinimumGradeYear.Int64)
		snapshot.MinimumGradeYear = &value
	}
	if classRow.MaximumGradeYear.Valid {
		value := uint16(classRow.MaximumGradeYear.Int64)
		snapshot.MaximumGradeYear = &value
	}

	if err := r.loadMajorScopes(ctx, teachingClassID, snapshot); err != nil {
		return nil, err
	}
	if err := r.loadPrerequisites(ctx, studentID, courseID, snapshot); err != nil {
		return nil, err
	}
	if err := r.loadSchedules(ctx, studentID, termID, teachingClassID, snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (r *EligibilityStore) loadMajorScopes(
	ctx context.Context,
	teachingClassID uint64,
	snapshot *enrollment.EligibilitySnapshot,
) error {
	var rows []struct {
		MajorID   uint64
		ScopeType string
	}
	if err := r.db.WithContext(ctx).
		Table("teaching_class_major_scope").
		Select("major_id, scope_type").
		Where("teaching_class_id = ?", teachingClassID).
		Find(&rows).Error; err != nil {
		return err
	}
	snapshot.MajorScopes = make([]enrollment.MajorScope, 0, len(rows))
	for _, row := range rows {
		snapshot.MajorScopes = append(snapshot.MajorScopes, enrollment.MajorScope{
			MajorID: row.MajorID,
			Type:    enrollment.MajorScopeType(row.ScopeType),
		})
	}
	return nil
}

func (r *EligibilityStore) loadPrerequisites(
	ctx context.Context,
	studentID uint64,
	courseID uint64,
	snapshot *enrollment.EligibilitySnapshot,
) error {
	var requirements []struct {
		PrerequisiteCourseID uint64
		MinimumScore         sql.NullFloat64
	}
	if err := r.db.WithContext(ctx).
		Table("course_prerequisite").
		Select("prerequisite_course_id, minimum_score").
		Where("course_id = ?", courseID).
		Find(&requirements).Error; err != nil {
		return err
	}
	snapshot.Prerequisites = make([]enrollment.PrerequisiteRequirement, 0, len(requirements))
	courseIDs := make([]uint64, 0, len(requirements))
	for _, row := range requirements {
		requirement := enrollment.PrerequisiteRequirement{
			CourseID: row.PrerequisiteCourseID,
		}
		if row.MinimumScore.Valid {
			score := row.MinimumScore.Float64
			requirement.MinimumScore = &score
		}
		snapshot.Prerequisites = append(snapshot.Prerequisites, requirement)
		courseIDs = append(courseIDs, row.PrerequisiteCourseID)
	}
	if len(courseIDs) == 0 {
		return nil
	}

	var histories []struct {
		CourseID uint64
		Score    sql.NullFloat64
		Result   string
	}
	if err := r.db.WithContext(ctx).
		Table("student_course_history").
		Select("course_id, score, result").
		Where("student_id = ? AND course_id IN ?", studentID, courseIDs).
		Find(&histories).Error; err != nil {
		return err
	}
	snapshot.Achievements = make([]enrollment.CourseAchievement, 0, len(histories))
	for _, row := range histories {
		achievement := enrollment.CourseAchievement{
			CourseID: row.CourseID,
			Passed:   row.Result == "passed" || row.Result == "exempted",
		}
		if row.Score.Valid {
			score := row.Score.Float64
			achievement.Score = &score
		}
		snapshot.Achievements = append(snapshot.Achievements, achievement)
	}
	return nil
}

func (r *EligibilityStore) loadSchedules(
	ctx context.Context,
	studentID uint64,
	termID uint64,
	teachingClassID uint64,
	snapshot *enrollment.EligibilitySnapshot,
) error {
	var targetRows []scheduleRow
	if err := r.db.WithContext(ctx).
		Table("teaching_class_schedule").
		Select("day_of_week, start_week, end_week, start_section, end_section").
		Where("teaching_class_id = ?", teachingClassID).
		Find(&targetRows).Error; err != nil {
		return err
	}
	var enrolledRows []scheduleRow
	if err := r.db.WithContext(ctx).
		Table("student_course_enrollment AS sce").
		Select("tcs.day_of_week, tcs.start_week, tcs.end_week, tcs.start_section, tcs.end_section").
		Joins("JOIN teaching_class_schedule tcs ON tcs.teaching_class_id = sce.teaching_class_id").
		Where(
			"sce.student_id = ? AND sce.term_id = ? AND sce.state = ? AND sce.teaching_class_id <> ?",
			studentID,
			termID,
			string(enrollment.EnrollmentStateEnrolled),
			teachingClassID,
		).
		Find(&enrolledRows).Error; err != nil {
		return err
	}
	snapshot.TargetSchedules = scheduleRowsToDomain(targetRows)
	snapshot.EnrolledSchedules = scheduleRowsToDomain(enrolledRows)
	return nil
}

func scheduleRowsToDomain(rows []scheduleRow) []enrollment.ScheduleSlot {
	slots := make([]enrollment.ScheduleSlot, 0, len(rows))
	for _, row := range rows {
		slots = append(slots, enrollment.ScheduleSlot{
			DayOfWeek:    row.DayOfWeek,
			StartWeek:    row.StartWeek,
			EndWeek:      row.EndWeek,
			StartSection: row.StartSection,
			EndSection:   row.EndSection,
		})
	}
	return slots
}
