package catalogrepo

import (
	"context"
	"strings"
	"time"

	applicationcatalog "prizeforge/internal/catalog/application"
	"prizeforge/internal/catalog/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type teachingClassRow struct {
	ID          uint64 `gorm:"column:id;primaryKey"`
	ClassCode   string `gorm:"column:class_code"`
	TermID      uint64 `gorm:"column:term_id"`
	CourseID    uint64 `gorm:"column:course_id"`
	TeacherName string `gorm:"column:teacher_name"`
	Location    string `gorm:"column:location"`
	Capacity    uint32 `gorm:"column:capacity"`
	// selected_count is an enrollment-owned projection. Catalog reads it for
	// capacity validation and responses but never writes it.
	SelectedCount    uint32    `gorm:"column:selected_count;->"`
	MinimumGradeYear *uint16   `gorm:"column:minimum_grade_year"`
	MaximumGradeYear *uint16   `gorm:"column:maximum_grade_year"`
	State            string    `gorm:"column:state"`
	CreateTime       time.Time `gorm:"column:create_time"`
	UpdateTime       time.Time `gorm:"column:update_time"`

	CourseCode           string  `gorm:"column:course_code;->"`
	CourseName           string  `gorm:"column:course_name;->"`
	Credits              float64 `gorm:"column:credits;->"`
	Introduction         string  `gorm:"column:introduction;->"`
	Tags                 []byte  `gorm:"column:tags;->"`
	PreviewVideoID       *uint64 `gorm:"column:preview_video_id;->"`
	PreviewVideoTitle    *string `gorm:"column:preview_video_title;->"`
	PreviewVideoDuration *uint64 `gorm:"column:preview_video_duration_ms;->"`
}

func (teachingClassRow) TableName() string { return "teaching_class" }

type scheduleRow struct {
	ID              uint64 `gorm:"column:id;primaryKey"`
	TeachingClassID uint64 `gorm:"column:teaching_class_id"`
	DayOfWeek       uint8  `gorm:"column:day_of_week"`
	StartWeek       uint8  `gorm:"column:start_week"`
	EndWeek         uint8  `gorm:"column:end_week"`
	StartSection    uint8  `gorm:"column:start_section"`
	EndSection      uint8  `gorm:"column:end_section"`
}

func (scheduleRow) TableName() string { return "teaching_class_schedule" }

type majorScopeRow struct {
	ID              uint64 `gorm:"column:id;primaryKey"`
	TeachingClassID uint64 `gorm:"column:teaching_class_id"`
}

func (majorScopeRow) TableName() string { return "teaching_class_major_scope" }

func (r teachingClassRow) aggregate() catalog.TeachingClass {
	return catalog.TeachingClass{
		ID: r.ID, ClassCode: r.ClassCode, TermID: r.TermID, CourseID: r.CourseID,
		TeacherName: r.TeacherName, Location: r.Location, Capacity: r.Capacity,
		SelectedCount: r.SelectedCount, MinimumGradeYear: r.MinimumGradeYear,
		MaximumGradeYear: r.MaximumGradeYear, State: catalog.TeachingClassState(r.State),
		Schedules: []catalog.Schedule{}, CreateTime: r.CreateTime, UpdateTime: r.UpdateTime,
	}
}

func (r teachingClassRow) view() applicationcatalog.TeachingClassView {
	view := applicationcatalog.TeachingClassView{
		ID: r.ID, ClassCode: r.ClassCode, TermID: r.TermID, CourseID: r.CourseID,
		CourseCode: r.CourseCode, CourseName: r.CourseName, Credits: r.Credits,
		Introduction: r.Introduction, Tags: decodeTags(r.Tags),
		TeacherName: r.TeacherName, Location: r.Location, Capacity: r.Capacity,
		SelectedCount: r.SelectedCount, MinimumGradeYear: r.MinimumGradeYear,
		MaximumGradeYear: r.MaximumGradeYear, State: catalog.TeachingClassState(r.State),
		Schedules: []catalog.Schedule{}, CreateTime: r.CreateTime, UpdateTime: r.UpdateTime,
	}
	if r.PreviewVideoID != nil {
		view.PreviewVideo = &applicationcatalog.CourseVideoView{
			ID: *r.PreviewVideoID, Title: valueOrEmpty(r.PreviewVideoTitle),
			DurationMS: r.PreviewVideoDuration,
		}
	}
	return view
}

const teachingClassSelect = `
	tc.id, tc.class_code, tc.term_id, tc.course_id, tc.teacher_name, tc.location,
	tc.capacity, tc.selected_count, tc.minimum_grade_year, tc.maximum_grade_year,
	tc.state, tc.create_time, tc.update_time,
	c.course_code, c.course_name, c.credits, c.introduction, c.tags,
	cv.id AS preview_video_id, cv.title AS preview_video_title,
	cv.duration_ms AS preview_video_duration_ms`

func (r *Repository) ListTeachingClasses(ctx context.Context, termID uint64, keyword string) ([]applicationcatalog.TeachingClassView, error) {
	// 目录列表只投影已就绪的第 0 位预览视频，上传中的对象不会暴露给学生端。
	query := r.dbFor(ctx).Table("teaching_class AS tc").
		Select(teachingClassSelect).
		Joins("JOIN course AS c ON c.id = tc.course_id").
		Joins("LEFT JOIN course_video AS cv ON cv.course_id = c.id AND cv.video_kind = ? AND cv.sort_order = 0 AND cv.status = ?", string(catalog.CourseVideoKindPreview), string(catalog.CourseVideoStatusReady))
	if termID > 0 {
		query = query.Where("tc.term_id = ?", termID)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("tc.class_code LIKE ? OR c.course_code LIKE ? OR c.course_name LIKE ? OR tc.teacher_name LIKE ?", like, like, like, like)
	}
	var rows []teachingClassRow
	if err := query.Order("tc.term_id DESC, tc.class_code ASC, tc.id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return r.hydrateTeachingClasses(ctx, rows)
}

func (r *Repository) ListStudentCatalog(ctx context.Context, query applicationcatalog.StudentCatalogQuery) ([]applicationcatalog.TeachingClassView, error) {
	db := r.dbFor(ctx).Table("selection_round_class AS src").
		Select(teachingClassSelect).
		Joins("JOIN selection_round AS sr ON sr.id = src.round_id").
		Joins("JOIN teaching_class AS tc ON tc.id = src.teaching_class_id").
		Joins("JOIN course AS c ON c.id = tc.course_id").
		Joins("LEFT JOIN course_video AS cv ON cv.course_id = c.id AND cv.video_kind = ? AND cv.sort_order = 0 AND cv.status = ?", string(catalog.CourseVideoKindPreview), string(catalog.CourseVideoStatusReady)).
		Where("src.round_id = ? AND src.state = ? AND sr.state = ? AND sr.start_time <= NOW(3) AND sr.end_time > NOW(3) AND tc.state = ?", query.RoundID, "open", "open", string(catalog.TeachingClassStateOpen))
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("tc.class_code LIKE ? OR c.course_code LIKE ? OR c.course_name LIKE ? OR tc.teacher_name LIKE ?", like, like, like, like)
	}
	var rows []teachingClassRow
	if err := db.Order("c.course_code ASC, tc.class_code ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return r.hydrateTeachingClasses(ctx, rows)
}

func (r *Repository) GetTeachingClass(ctx context.Context, id uint64) (*applicationcatalog.TeachingClassView, error) {
	var row teachingClassRow
	err := r.dbFor(ctx).Table("teaching_class AS tc").
		Select(teachingClassSelect).
		Joins("JOIN course AS c ON c.id = tc.course_id").
		Joins("LEFT JOIN course_video AS cv ON cv.course_id = c.id AND cv.video_kind = ? AND cv.sort_order = 0 AND cv.status = ?", string(catalog.CourseVideoKindPreview), string(catalog.CourseVideoStatusReady)).
		Where("tc.id = ?", id).Take(&row).Error
	if err != nil {
		return nil, normalizeDBError(err)
	}
	items, err := r.hydrateTeachingClassViews(ctx, []teachingClassRow{row})
	if err != nil {
		return nil, err
	}
	return &items[0], nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (r *Repository) GetTeachingClassForUpdate(ctx context.Context, id uint64) (*catalog.TeachingClass, error) {
	var row teachingClassRow
	if err := r.dbFor(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Take(&row, "id = ?", id).Error; err != nil {
		return nil, normalizeDBError(err)
	}
	class := row.aggregate()
	if err := r.hydrateTeachingClassAggregate(ctx, &class); err != nil {
		return nil, err
	}
	return &class, nil
}

func (r *Repository) hydrateTeachingClassAggregate(ctx context.Context, class *catalog.TeachingClass) error {
	var schedules []scheduleRow
	if err := r.dbFor(ctx).Where("teaching_class_id = ?", class.ID).
		Order("day_of_week ASC, start_section ASC").Find(&schedules).Error; err != nil {
		return err
	}
	for _, row := range schedules {
		class.Schedules = append(class.Schedules, scheduleFromRow(row))
	}
	return nil
}

func (r *Repository) hydrateTeachingClassViews(ctx context.Context, rows []teachingClassRow) ([]applicationcatalog.TeachingClassView, error) {
	items := make([]applicationcatalog.TeachingClassView, 0, len(rows))
	if len(rows) == 0 {
		return items, nil
	}
	ids := make([]uint64, 0, len(rows))
	index := make(map[uint64]int, len(rows))
	for _, row := range rows {
		index[row.ID] = len(items)
		ids = append(ids, row.ID)
		items = append(items, row.view())
	}
	var schedules []scheduleRow
	if err := r.dbFor(ctx).Where("teaching_class_id IN ?", ids).
		Order("teaching_class_id ASC, day_of_week ASC, start_section ASC").Find(&schedules).Error; err != nil {
		return nil, err
	}
	for _, row := range schedules {
		position, ok := index[row.TeachingClassID]
		if !ok {
			continue
		}
		items[position].Schedules = append(items[position].Schedules, scheduleFromRow(row))
	}
	return items, nil
}

func scheduleFromRow(row scheduleRow) catalog.Schedule {
	return catalog.Schedule{
		ID: row.ID, TeachingClassID: row.TeachingClassID, DayOfWeek: row.DayOfWeek,
		StartWeek: row.StartWeek, EndWeek: row.EndWeek,
		StartSection: row.StartSection, EndSection: row.EndSection,
	}
}

func (r *Repository) hydrateTeachingClasses(ctx context.Context, rows []teachingClassRow) ([]applicationcatalog.TeachingClassView, error) {
	items, err := r.hydrateTeachingClassViews(ctx, rows)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) InsertTeachingClass(ctx context.Context, class *catalog.TeachingClass) error {
	row := teachingClassRow{
		ClassCode: class.ClassCode, TermID: class.TermID, CourseID: class.CourseID,
		TeacherName: class.TeacherName, Location: class.Location, Capacity: class.Capacity,
		MinimumGradeYear: class.MinimumGradeYear,
		MaximumGradeYear: class.MaximumGradeYear, State: string(class.State),
	}
	if err := r.dbFor(ctx).Create(&row).Error; err != nil {
		return normalizeDBError(err)
	}
	if err := replaceSchedules(r.dbFor(ctx), row.ID, class.Schedules); err != nil {
		return err
	}
	class.ID = row.ID
	class.CreateTime = row.CreateTime
	class.UpdateTime = row.UpdateTime
	return nil
}

func (r *Repository) SaveTeachingClass(ctx context.Context, class *catalog.TeachingClass) error {
	result := r.dbFor(ctx).Model(&teachingClassRow{}).
		Where("id = ? AND state = ?", class.ID, string(catalog.TeachingClassStatePlanned)).
		Updates(map[string]interface{}{
			"class_code": class.ClassCode, "term_id": class.TermID, "course_id": class.CourseID,
			"teacher_name": class.TeacherName, "location": class.Location, "capacity": class.Capacity,
			"minimum_grade_year": class.MinimumGradeYear, "maximum_grade_year": class.MaximumGradeYear,
			"update_time": gorm.Expr("GREATEST(DATE_ADD(update_time, INTERVAL 1 MILLISECOND), NOW(3))"),
		})
	if err := requireConditionalWrite(result); err != nil {
		return err
	}
	return replaceSchedules(r.dbFor(ctx), class.ID, class.Schedules)
}

func replaceSchedules(tx *gorm.DB, teachingClassID uint64, schedules []catalog.Schedule) error {
	if err := tx.Where("teaching_class_id = ?", teachingClassID).Delete(&scheduleRow{}).Error; err != nil {
		return err
	}
	if len(schedules) == 0 {
		return nil
	}
	rows := make([]scheduleRow, 0, len(schedules))
	for _, schedule := range schedules {
		rows = append(rows, scheduleRow{
			TeachingClassID: teachingClassID, DayOfWeek: schedule.DayOfWeek,
			StartWeek: schedule.StartWeek, EndWeek: schedule.EndWeek,
			StartSection: schedule.StartSection, EndSection: schedule.EndSection,
		})
	}
	return tx.Create(&rows).Error
}

func (r *Repository) RemoveTeachingClass(ctx context.Context, id uint64) error {
	db := r.dbFor(ctx)
	if err := db.Where("teaching_class_id = ?", id).Delete(&scheduleRow{}).Error; err != nil {
		return err
	}
	if err := db.Where("teaching_class_id = ?", id).Delete(&majorScopeRow{}).Error; err != nil {
		return err
	}
	result := db.Delete(&teachingClassRow{}, "id = ? AND state = ?", id, string(catalog.TeachingClassStatePlanned))
	return requireConditionalWrite(result)
}

func (r *Repository) InspectTeachingClassUsage(ctx context.Context, id uint64) (catalog.TeachingClassUsage, error) {
	db := r.dbFor(ctx)
	var usage catalog.TeachingClassUsage
	queries := []struct {
		table  string
		target *int64
	}{
		{table: "selection_round_class", target: &usage.RoundBindingCount},
		{table: "selection_application", target: &usage.ApplicationCount},
		{table: "selection_waitlist", target: &usage.WaitlistCount},
		{table: "student_course_enrollment", target: &usage.EnrollmentCount},
	}
	for _, query := range queries {
		if err := db.Table(query.table).Where("teaching_class_id = ?", id).Count(query.target).Error; err != nil {
			return catalog.TeachingClassUsage{}, err
		}
	}
	return usage, nil
}
