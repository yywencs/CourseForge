package enrollmentrepo

import "fmt"

// SelectionResultStreamKey 保存 Redis 原子选课决策等待 MySQL 投影的事件。
const SelectionResultStreamKey = "courseforge:selection:result:stream"

const selectionResultStreamKey = SelectionResultStreamKey

func studentCreditKey(roundID, studentID uint64) string {
	return fmt.Sprintf("courseforge:selection:quota:credit:%d:%d", roundID, studentID)
}

func studentCourseQuotaKey(roundID, studentID uint64) string {
	return fmt.Sprintf("courseforge:selection:quota:course:%d:%d", roundID, studentID)
}

func teachingClassSeatKey(teachingClassID uint64) string {
	return fmt.Sprintf("courseforge:selection:class:seat:%d", teachingClassID)
}

func pendingApplicationKey(roundID, studentID uint64) string {
	return fmt.Sprintf("courseforge:selection:pending:%d:%d", roundID, studentID)
}

func requestResultKey(roundID, studentID uint64, requestID string) string {
	return fmt.Sprintf("courseforge:selection:result:%d:%d:%s", roundID, studentID, requestID)
}

func applicationLookupKey(applicationID string) string {
	return fmt.Sprintf("courseforge:selection:application:%s", applicationID)
}

func droppedEnrollmentKey(enrollmentID string) string {
	return fmt.Sprintf("courseforge:selection:dropped:%s", enrollmentID)
}

func roundSnapshotKey(roundID uint64, version string) string {
	return fmt.Sprintf("courseforge:selection:round:%d:%s", roundID, version)
}

func roundOpenVersionKey(roundID uint64) string {
	return fmt.Sprintf("courseforge:selection:round:%d:open_version", roundID)
}

func teachingClassSnapshotKey(roundID uint64, version string, teachingClassID uint64) string {
	return fmt.Sprintf("courseforge:selection:class:%d:%s:%d", roundID, version, teachingClassID)
}

func teachingClassScheduleKey(teachingClassID uint64) string {
	return fmt.Sprintf("courseforge:selection:class:schedule:%d", teachingClassID)
}

func studentScheduleKey(roundID, studentID uint64) string {
	return fmt.Sprintf("courseforge:selection:student:schedule:%d:%d", roundID, studentID)
}

func studentCourseSelectionKey(roundID, studentID uint64) string {
	return fmt.Sprintf("courseforge:selection:student:courses:%d:%d", roundID, studentID)
}
