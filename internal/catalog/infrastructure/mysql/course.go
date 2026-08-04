package catalogrepo

import (
	"context"
	"strings"
	"time"

	"github.com/yywencs/courseforge/internal/catalog/domain"

	"gorm.io/gorm/clause"
)

type courseRow struct {
	ID           uint64    `gorm:"column:id;primaryKey"`
	CourseCode   string    `gorm:"column:course_code"`
	CourseName   string    `gorm:"column:course_name"`
	Credits      float64   `gorm:"column:credits"`
	Introduction string    `gorm:"column:introduction"`
	Tags         []byte    `gorm:"column:tags"`
	CreateTime   time.Time `gorm:"column:create_time"`
	UpdateTime   time.Time `gorm:"column:update_time"`
}

func (courseRow) TableName() string { return "course" }

func (r courseRow) domain() catalog.Course {
	return catalog.Course{
		ID: r.ID, CourseCode: r.CourseCode, CourseName: r.CourseName,
		Credits: r.Credits, Introduction: r.Introduction, Tags: decodeTags(r.Tags),
		CreateTime: r.CreateTime, UpdateTime: r.UpdateTime,
	}
}

func (r *Repository) ListCourses(ctx context.Context, keyword string) ([]catalog.Course, error) {
	query := r.dbFor(ctx).Model(&courseRow{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("course_code LIKE ? OR course_name LIKE ?", like, like)
	}
	var rows []courseRow
	if err := query.Order("course_code ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]catalog.Course, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.domain())
	}
	return items, nil
}

func (r *Repository) GetCourse(ctx context.Context, id uint64) (*catalog.Course, error) {
	return r.getCourse(ctx, id, false)
}

func (r *Repository) GetCourseForUpdate(ctx context.Context, id uint64) (*catalog.Course, error) {
	return r.getCourse(ctx, id, true)
}

func (r *Repository) getCourse(ctx context.Context, id uint64, forUpdate bool) (*catalog.Course, error) {
	query := r.dbFor(ctx)
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row courseRow
	if err := query.Take(&row, "id = ?", id).Error; err != nil {
		return nil, normalizeDBError(err)
	}
	item := row.domain()
	return &item, nil
}

func (r *Repository) InsertCourse(ctx context.Context, course *catalog.Course) error {
	tags, err := encodeTags(course.Tags)
	if err != nil {
		return err
	}
	row := courseRow{
		CourseCode: course.CourseCode, CourseName: course.CourseName, Credits: course.Credits,
		Introduction: course.Introduction, Tags: tags,
	}
	if err := r.dbFor(ctx).Create(&row).Error; err != nil {
		return normalizeDBError(err)
	}
	*course = row.domain()
	return nil
}

func (r *Repository) SaveCourse(ctx context.Context, course *catalog.Course) error {
	tags, err := encodeTags(course.Tags)
	if err != nil {
		return err
	}
	return normalizeDBError(r.dbFor(ctx).Model(&courseRow{}).Where("id = ?", course.ID).Updates(map[string]interface{}{
		"course_code": course.CourseCode, "course_name": course.CourseName,
		"credits": course.Credits, "introduction": course.Introduction,
		"tags": tags,
	}).Error)
}

func (r *Repository) RemoveCourse(ctx context.Context, id uint64) error {
	result := r.dbFor(ctx).Delete(&courseRow{}, "id = ?", id)
	if result.Error != nil {
		return normalizeDBError(result.Error)
	}
	if result.RowsAffected != 1 {
		return catalog.ErrNotFound
	}
	return nil
}

func (r *Repository) InspectCourseUsage(ctx context.Context, id uint64) (catalog.CourseUsage, error) {
	db := r.dbFor(ctx)
	var usage catalog.CourseUsage
	queries := []struct {
		table  string
		where  string
		args   []interface{}
		target *int64
	}{
		{table: "teaching_class", where: "course_id = ?", args: []interface{}{id}, target: &usage.TeachingClassCount},
		{table: "teaching_class", where: "course_id = ? AND state <> ?", args: []interface{}{id, string(catalog.TeachingClassStatePlanned)}, target: &usage.NonPlannedTeachingClassCount},
		{table: "course_video", where: "course_id = ?", args: []interface{}{id}, target: &usage.CourseVideoCount},
		{table: "course_prerequisite", where: "course_id = ? OR prerequisite_course_id = ?", args: []interface{}{id, id}, target: &usage.PrerequisiteCount},
		{table: "student_course_history", where: "course_id = ?", args: []interface{}{id}, target: &usage.StudentHistoryCount},
	}
	for _, query := range queries {
		if err := db.Table(query.table).Where(query.where, query.args...).Count(query.target).Error; err != nil {
			return catalog.CourseUsage{}, err
		}
	}
	return usage, nil
}
