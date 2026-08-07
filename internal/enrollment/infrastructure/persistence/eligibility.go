package enrollmentrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	enrollmentapp "github.com/yywencs/courseforge/internal/enrollment/application"
	"github.com/yywencs/courseforge/internal/enrollment/domain"

	"gorm.io/gorm"
)

// LoadRoundWarmupSnapshot 批量加载轮次、教学班以及全部静态限制。
// 查询次数只随规则种类增长，不随学生数量增长。
func (r *EligibilityStore) LoadRoundWarmupSnapshot(
	ctx context.Context,
	roundID uint64,
) (*enrollmentapp.RoundWarmupSnapshot, error) {
	if roundID == 0 {
		return nil, enrollment.ErrInvalidParams
	}
	var roundRow struct {
		ID         uint64
		TermID     uint64
		RoundCode  string
		RoundName  string
		StartTime  time.Time
		EndTime    time.Time
		State      string
		CreateTime time.Time
		UpdateTime time.Time
	}
	err := r.db.WithContext(ctx).Table("selection_round").Where("id = ?", roundID).Take(&roundRow).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, enrollment.ErrRecordNotFound
	}
	if err != nil {
		return nil, err
	}
	snapshot := &enrollmentapp.RoundWarmupSnapshot{Round: enrollment.SelectionRound{
		ID: roundRow.ID, TermID: roundRow.TermID, RoundCode: roundRow.RoundCode,
		RoundName: roundRow.RoundName, StartTime: roundRow.StartTime, EndTime: roundRow.EndTime,
		State: enrollment.SelectionRoundState(roundRow.State), CreateTime: roundRow.CreateTime,
		UpdateTime: roundRow.UpdateTime,
	}}

	var classRows []struct {
		ID               uint64
		CourseID         uint64
		Credits          string
		Capacity         uint32
		SelectedCount    uint32
		MinimumGradeYear sql.NullInt64
		MaximumGradeYear sql.NullInt64
	}
	err = r.db.WithContext(ctx).Table("selection_round_class AS src").
		Select(`tc.id, tc.course_id, CAST(c.credits AS CHAR) AS credits, tc.capacity,
			tc.selected_count, tc.minimum_grade_year, tc.maximum_grade_year`).
		Joins("JOIN teaching_class AS tc ON tc.id = src.teaching_class_id").
		Joins("JOIN course AS c ON c.id = tc.course_id").
		Where("src.round_id = ? AND src.state = ? AND tc.term_id = ?", roundID, "open", roundRow.TermID).
		Order("tc.id ASC").Scan(&classRows).Error
	if err != nil {
		return nil, err
	}
	classByID := make(map[uint64]*enrollmentapp.WarmupClass, len(classRows))
	courseClasses := make(map[uint64][]*enrollmentapp.WarmupClass)
	classIDs := make([]uint64, 0, len(classRows))
	courseIDs := make([]uint64, 0, len(classRows))
	for _, row := range classRows {
		credits, parseErr := creditFromDecimal(row.Credits)
		if parseErr != nil {
			return nil, fmt.Errorf("解析教学班 %d 学分: %w", row.ID, parseErr)
		}
		item := enrollmentapp.WarmupClass{
			ID: row.ID, CourseID: row.CourseID, Credits: credits,
			Capacity: row.Capacity, SelectedCount: row.SelectedCount,
		}
		if row.MinimumGradeYear.Valid {
			value := uint16(row.MinimumGradeYear.Int64)
			item.MinimumGradeYear = &value
		}
		if row.MaximumGradeYear.Valid {
			value := uint16(row.MaximumGradeYear.Int64)
			item.MaximumGradeYear = &value
		}
		snapshot.Classes = append(snapshot.Classes, item)
		classIDs = append(classIDs, row.ID)
		courseIDs = append(courseIDs, row.CourseID)
	}
	for i := range snapshot.Classes {
		item := &snapshot.Classes[i]
		classByID[item.ID] = item
		courseClasses[item.CourseID] = append(courseClasses[item.CourseID], item)
	}
	if len(classIDs) == 0 {
		return snapshot, nil
	}

	var scopeRows []struct {
		TeachingClassID uint64
		MajorID         uint64
		ScopeType       string
	}
	if err := r.db.WithContext(ctx).Table("teaching_class_major_scope").
		Select("teaching_class_id, major_id, scope_type").Where("teaching_class_id IN ?", classIDs).
		Order("teaching_class_id, id").Scan(&scopeRows).Error; err != nil {
		return nil, err
	}
	for _, row := range scopeRows {
		if item := classByID[row.TeachingClassID]; item != nil {
			item.MajorScopes = append(item.MajorScopes, enrollment.MajorScope{
				MajorID: row.MajorID, Type: enrollment.MajorScopeType(row.ScopeType),
			})
		}
	}

	var prerequisiteRows []struct {
		CourseID             uint64
		PrerequisiteCourseID uint64
		MinimumScore         sql.NullFloat64
	}
	if err := r.db.WithContext(ctx).Table("course_prerequisite").
		Select("course_id, prerequisite_course_id, minimum_score").Where("course_id IN ?", courseIDs).
		Order("course_id, id").Scan(&prerequisiteRows).Error; err != nil {
		return nil, err
	}
	for _, row := range prerequisiteRows {
		requirement := enrollment.PrerequisiteRequirement{CourseID: row.PrerequisiteCourseID}
		if row.MinimumScore.Valid {
			value := row.MinimumScore.Float64
			requirement.MinimumScore = &value
		}
		for _, item := range courseClasses[row.CourseID] {
			item.Prerequisites = append(item.Prerequisites, requirement)
		}
	}

	var classScheduleRows []struct {
		TeachingClassID uint64
		DayOfWeek       uint8
		StartWeek       uint8
		EndWeek         uint8
		StartSection    uint8
		EndSection      uint8
	}
	if err := r.db.WithContext(ctx).Table("teaching_class_schedule").
		Select("teaching_class_id, day_of_week, start_week, end_week, start_section, end_section").
		Where("teaching_class_id IN ?", classIDs).
		Order("teaching_class_id, id").Scan(&classScheduleRows).Error; err != nil {
		return nil, err
	}
	for _, row := range classScheduleRows {
		if item := classByID[row.TeachingClassID]; item != nil {
			item.Schedules = append(item.Schedules, enrollment.ScheduleSlot{
				DayOfWeek: row.DayOfWeek, StartWeek: row.StartWeek, EndWeek: row.EndWeek,
				StartSection: row.StartSection, EndSection: row.EndSection,
			})
		}
	}
	return snapshot, nil
}

// ListRoundWarmupStudents 按学生 ID 游标批量加载档案、额度和历史成绩。
func (r *EligibilityStore) ListRoundWarmupStudents(
	ctx context.Context,
	roundID uint64,
	afterStudentID uint64,
	limit int,
) ([]enrollmentapp.WarmupStudent, error) {
	if roundID == 0 || limit <= 0 {
		return nil, enrollment.ErrInvalidParams
	}
	var rows []struct {
		ID                  uint64
		MajorID             uint64
		GradeYear           uint16
		State               string
		TermID              uint64
		CreditLimit         string
		SelectedCredits     string
		CourseLimit         uint16
		SelectedCourseCount uint16
	}
	err := r.db.WithContext(ctx).Table("student_selection_quota AS q").
		Select(`s.id, s.major_id, s.grade_year, s.state, q.term_id,
			CAST(q.credit_limit AS CHAR) AS credit_limit,
			CAST(q.selected_credits AS CHAR) AS selected_credits,
			q.course_limit, q.selected_course_count`).
		Joins("JOIN student AS s ON s.id = q.student_id").
		Where("q.round_id = ? AND s.id > ?", roundID, afterStudentID).
		Order("s.id ASC").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	students := make([]enrollmentapp.WarmupStudent, 0, len(rows))
	studentIDs := make([]uint64, 0, len(rows))
	studentIndex := make(map[uint64]int, len(rows))
	for _, row := range rows {
		creditLimit, parseErr := creditFromDecimal(row.CreditLimit)
		if parseErr != nil {
			return nil, fmt.Errorf("解析学生 %d 学分上限: %w", row.ID, parseErr)
		}
		selectedCredits, parseErr := creditFromDecimalAllowZero(row.SelectedCredits)
		if parseErr != nil {
			return nil, fmt.Errorf("解析学生 %d 已选学分: %w", row.ID, parseErr)
		}
		studentIndex[row.ID] = len(students)
		studentIDs = append(studentIDs, row.ID)
		students = append(students, enrollmentapp.WarmupStudent{
			Profile: enrollment.StudentProfile{ID: row.ID, MajorID: row.MajorID, GradeYear: row.GradeYear, State: enrollment.StudentState(row.State)},
			Quota: enrollment.StudentSelectionQuota{RoundID: roundID, TermID: row.TermID, StudentID: row.ID,
				CreditLimit: creditLimit, SelectedCredits: selectedCredits,
				CourseLimit: row.CourseLimit, SelectedCourseCount: row.SelectedCourseCount},
		})
	}
	if len(studentIDs) == 0 {
		return students, nil
	}
	var historyRows []struct {
		StudentID uint64
		CourseID  uint64
		Score     sql.NullFloat64
		Result    string
	}
	if err := r.db.WithContext(ctx).Table("student_course_history AS sch").
		Distinct("sch.student_id, sch.course_id, sch.score, sch.result").
		Joins("JOIN course_prerequisite AS cp ON cp.prerequisite_course_id = sch.course_id").
		Joins("JOIN teaching_class AS tc ON tc.course_id = cp.course_id").
		Joins("JOIN selection_round_class AS src ON src.teaching_class_id = tc.id AND src.round_id = ?", roundID).
		Where("sch.student_id IN ?", studentIDs).
		Order("sch.student_id, sch.course_id").Scan(&historyRows).Error; err != nil {
		return nil, err
	}
	for _, row := range historyRows {
		achievement := enrollment.CourseAchievement{
			CourseID: row.CourseID, Passed: row.Result == "passed" || row.Result == "exempted",
		}
		if row.Score.Valid {
			value := row.Score.Float64
			achievement.Score = &value
		}
		index, ok := studentIndex[row.StudentID]
		if ok {
			students[index].Achievements = append(students[index].Achievements, achievement)
		}
	}

	var enrollmentRows []struct {
		StudentID       uint64
		ApplicationID   string
		CourseID        uint64
		TeachingClassID uint64
		DayOfWeek       sql.NullInt64
		StartWeek       sql.NullInt64
		EndWeek         sql.NullInt64
		StartSection    sql.NullInt64
		EndSection      sql.NullInt64
	}
	if err := r.db.WithContext(ctx).Table("student_course_enrollment AS sce").
		Select(`sce.student_id, sce.application_id, sce.course_id, sce.teaching_class_id,
			tcs.day_of_week, tcs.start_week, tcs.end_week, tcs.start_section, tcs.end_section`).
		Joins("LEFT JOIN teaching_class_schedule AS tcs ON tcs.teaching_class_id = sce.teaching_class_id").
		Where("sce.student_id IN ? AND sce.term_id = ? AND sce.state = ?",
			studentIDs, rows[0].TermID, string(enrollment.EnrollmentStateEnrolled)).
		Order("sce.student_id, sce.application_id, tcs.id").Scan(&enrollmentRows).Error; err != nil {
		return nil, err
	}
	type enrollmentLocation struct {
		studentIndex    int
		enrollmentIndex int
	}
	locations := make(map[string]enrollmentLocation, len(enrollmentRows))
	for _, row := range enrollmentRows {
		index, ok := studentIndex[row.StudentID]
		if !ok {
			continue
		}
		location, exists := locations[row.ApplicationID]
		if !exists {
			students[index].Enrollments = append(students[index].Enrollments, enrollmentapp.WarmupEnrollment{
				ApplicationID: row.ApplicationID, CourseID: row.CourseID,
				TeachingClassID: row.TeachingClassID,
			})
			location = enrollmentLocation{studentIndex: index, enrollmentIndex: len(students[index].Enrollments) - 1}
			locations[row.ApplicationID] = location
		}
		if row.DayOfWeek.Valid && row.StartWeek.Valid && row.EndWeek.Valid &&
			row.StartSection.Valid && row.EndSection.Valid {
			item := &students[location.studentIndex].Enrollments[location.enrollmentIndex]
			item.Schedules = append(item.Schedules, enrollment.ScheduleSlot{
				DayOfWeek: uint8(row.DayOfWeek.Int64), StartWeek: uint8(row.StartWeek.Int64),
				EndWeek: uint8(row.EndWeek.Int64), StartSection: uint8(row.StartSection.Int64),
				EndSection: uint8(row.EndSection.Int64),
			})
		}
	}
	return students, nil
}
