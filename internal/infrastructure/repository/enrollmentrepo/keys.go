package enrollmentrepo

import "fmt"

const selectionResultStreamKey = "courseforge:selection:result:stream"

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

func selectedCourseGuardKey(termID, studentID, courseID uint64) string {
	return fmt.Sprintf("courseforge:selection:course:%d:%d:%d", termID, studentID, courseID)
}
