import { computed, readonly, shallowRef } from 'vue'

import {
  bindRoundClass,
  createCourse,
  createSelectionRound,
  createTeachingClass,
  deleteCourse,
  deleteSelectionRound,
  deleteTeachingClass,
  listCourses,
  listRoundClasses,
  listSelectionRounds,
  listTeachingClasses,
	getSelectionRoundWarmup,
	requestSelectionRoundWarmup,
	openSelectionRound,
  unbindRoundClass,
  updateCourse,
  updateSelectionRound,
  updateTeachingClass,
} from '@/api/catalog'
import type {
  Course,
  CourseDraft,
  RoundClassBinding,
	RoundWarmupStatus,
  SelectionRound,
  SelectionRoundDraft,
  TeachingClass,
  TeachingClassDraft,
} from '@/types/catalog'

export function useCatalogAdmin() {
  const courses = shallowRef<Course[]>([])
  const teachingClasses = shallowRef<TeachingClass[]>([])
  const rounds = shallowRef<SelectionRound[]>([])
  const bindings = shallowRef<RoundClassBinding[]>([])
	const warmupStatuses = shallowRef<Record<number, RoundWarmupStatus | null>>({})
  const loadingCount = shallowRef(0)
  const activeRoundId = shallowRef(0)
  const isLoading = computed(() => loadingCount.value > 0)

  async function tracked<T>(task: () => Promise<T>): Promise<T> {
    loadingCount.value += 1
    try {
      return await task()
    } finally {
      loadingCount.value -= 1
    }
  }

  async function refreshAll(termId?: number): Promise<void> {
    await tracked(async () => {
      const [courseResult, classResult, roundResult] = await Promise.all([
        listCourses(),
        listTeachingClasses(termId),
        listSelectionRounds(termId),
      ])
      courses.value = courseResult.items
      teachingClasses.value = classResult.items
      rounds.value = roundResult.items
		warmupStatuses.value = Object.fromEntries(roundResult.items.map((round) => [round.id, round.warmup_state ? {
			round_id: round.id, version: round.warmup_version ?? '', state: round.warmup_state,
			student_count: 0, eligible_count: 0, started_at: '',
		} : null]))
    })
  }

  async function saveCourse(draft: CourseDraft, id?: number): Promise<void> {
    await tracked(() => id ? updateCourse(id, draft) : createCourse(draft))
    courses.value = (await listCourses()).items
  }

  async function removeCourse(id: number): Promise<void> {
    await tracked(() => deleteCourse(id))
    courses.value = courses.value.filter((item) => item.id !== id)
  }

  async function saveTeachingClass(draft: TeachingClassDraft, id?: number): Promise<void> {
    await tracked(() => id ? updateTeachingClass(id, draft) : createTeachingClass(draft))
    teachingClasses.value = (await listTeachingClasses()).items
  }

  async function removeTeachingClass(id: number): Promise<void> {
    await tracked(() => deleteTeachingClass(id))
    teachingClasses.value = teachingClasses.value.filter((item) => item.id !== id)
  }

  async function saveRound(draft: SelectionRoundDraft, id?: number): Promise<void> {
    await tracked(() => id ? updateSelectionRound(id, draft) : createSelectionRound(draft))
    rounds.value = (await listSelectionRounds()).items
  }

  async function removeRound(id: number): Promise<void> {
    await tracked(() => deleteSelectionRound(id))
    rounds.value = rounds.value.filter((item) => item.id !== id)
  }

	async function warmupRound(id: number): Promise<void> {
		await tracked(() => requestSelectionRoundWarmup(id))
		warmupStatuses.value = { ...warmupStatuses.value, [id]: {
			round_id: id, version: '', state: 'queued', student_count: 0,
			eligible_count: 0, started_at: new Date().toISOString(),
		} }
	}

	async function refreshWarmupStatus(id: number): Promise<RoundWarmupStatus | null> {
		const status = await tracked(() => getSelectionRoundWarmup(id))
		warmupStatuses.value = { ...warmupStatuses.value, [id]: status }
		return status
	}

	async function openRound(id: number): Promise<void> {
		await tracked(() => openSelectionRound(id))
		rounds.value = (await listSelectionRounds()).items
	}

  async function loadBindings(roundId: number): Promise<void> {
    activeRoundId.value = roundId
    bindings.value = (await tracked(() => listRoundClasses(roundId))).items
  }

  async function addBinding(teachingClassId: number): Promise<void> {
    await tracked(() => bindRoundClass(activeRoundId.value, teachingClassId))
    bindings.value = (await listRoundClasses(activeRoundId.value)).items
  }

  async function removeBinding(teachingClassId: number): Promise<void> {
    await tracked(() => unbindRoundClass(activeRoundId.value, teachingClassId))
    bindings.value = bindings.value.filter((item) => item.teaching_class_id !== teachingClassId)
  }

  return {
    courses: readonly(courses),
    teachingClasses: readonly(teachingClasses),
    rounds: readonly(rounds),
    bindings: readonly(bindings),
		warmupStatuses: readonly(warmupStatuses),
    activeRoundId: readonly(activeRoundId),
    isLoading,
    refreshAll,
    saveCourse,
    removeCourse,
    saveTeachingClass,
    removeTeachingClass,
    saveRound,
    removeRound,
		warmupRound,
		refreshWarmupStatus,
		openRound,
    loadBindings,
    addBinding,
    removeBinding,
  }
}
