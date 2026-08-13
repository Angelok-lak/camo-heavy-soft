// Typed mirror of the F-09 HTTP contract. The front computes NOTHING:
// conflicts, severities and explanations arrive ready-made (C-26).

export type Severity = 'CRITICAL' | 'WARNING' | 'INFO'

export interface TimeSlot {
  Start: string
  End: string
}

export interface PartyRef {
  Kind: string
  ID: string
  Label: string
  Slot: TimeSlot
  Note: string
}

export interface Conflict {
  Kind: string
  Severity: Severity
  SubjectID: string
  Subject: string
  Overlap: TimeSlot | null
  Parties: PartyRef[] | null
  SameCauseAs: number | null
}

export interface Resource {
  ID: string
  Kind: 'VEHICLE' | 'ROOM' | 'TRAINING_PAD' | 'INSTRUCTOR'
  Label: string
  Archived: boolean
  Categories: string[] | null
}

export interface Enrollment {
  ID: string
  StudentName: string
  Category: string
  Closed: boolean
}

export interface LessonKind {
  ID: string
  Label: string
  RequiresVehicle: boolean
}

export interface Lesson {
  ID: string
  Slot: TimeSlot
  Resources: string[] | null
  Enrollments: string[] | null
  Cancelled: boolean
  MaxStudents: number | null
  Kinds: LessonKind[] | null
  conflicts: Conflict[]
}

// An exam session as the planner sees it. TravelTime arrives in Go
// nanoseconds; the display divides by 60e9 for minutes.
export interface PlanningExam {
  ID: string
  Slot: TimeSlot
  Resources: string[] | null
  PlaceLabel: string
  TravelTime: number
}

export interface PlanningView {
  period: TimeSlot
  lessons: Lesson[]
  exam_sessions: PlanningExam[]
  resources: Record<string, Resource>
  lesson_kinds: Record<string, LessonKind>
  enrollments: Record<string, Enrollment>
  opening_hours: unknown[]
  closures: TimeSlot[]
}

export interface Operation {
  id: string
  kind:
    | 'CREATE_LESSON'
    | 'MOVE_LESSON'
    | 'CANCEL_LESSON'
    | 'ASSIGN_RESOURCE'
    | 'UNASSIGN_RESOURCE'
    | 'PLACE_STUDENT'
    | 'REMOVE_STUDENT'
  lesson_id: string
  slot?: TimeSlot
  kind_ids?: string[]
  resource_id?: string
  enrollment_id?: string
  reason?: string
}

export interface EditSession {
  id: string
  user_id: string
  scope: TimeSlot
  status: string
  operations: Operation[] | null
  opened_at: string
  last_activity_at: string
}

export interface DraftView {
  lessons: (Omit<Lesson, 'conflicts'> & { conflicts?: undefined })[] | null
  pending: Operation[] | null
  rejected: { operation_id: string; reason: string }[] | null
  conflicts: Record<string, Conflict[]>
}

export interface SessionView {
  session: EditSession
  draft: DraftView
}

export interface LockHolder {
  holder_name: string
  holder_user_id: string
  opened_at: string
  session_id: string
}

// UUID v7 (C-02): time-ordered ids, generated client-side so a draft can
// reference a lesson that does not exist in storage yet.
export function uuidv7(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16))
  const ts = Date.now()
  bytes[0] = (ts / 2 ** 40) & 0xff
  bytes[1] = (ts / 2 ** 32) & 0xff
  bytes[2] = (ts / 2 ** 24) & 0xff
  bytes[3] = (ts / 2 ** 16) & 0xff
  bytes[4] = (ts / 2 ** 8) & 0xff
  bytes[5] = ts & 0xff
  bytes[6] = 0x70 | (bytes[6] & 0x0f)
  bytes[8] = 0x80 | (bytes[8] & 0x3f)
  const hex = [...bytes].map((b) => b.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

export class ApiError extends Error {
  status: number
  body: Record<string, unknown>

  constructor(status: number, body: Record<string, unknown>) {
    super(String(body.error ?? status))
    this.status = status
    this.body = body
  }
}

// F-17: the session lives in an HttpOnly cookie the browser sends by
// itself. Permissions arrive computed — the front decides nothing (C-26).
export interface UserContext {
  user_id: string
  name: string
  school_id: string
  profiles: ('MANAGEMENT' | 'OFFICE' | 'INSTRUCTOR')[]
  instructor_resource_id: string | null
  permissions: {
    edit_planning: boolean
    force_release: boolean
    manage_resources: boolean
    manage_people: boolean
    manage_settings: boolean
  }
}

export interface ResourceRow {
  id: string
  kind: 'VEHICLE' | 'ROOM' | 'TRAINING_PAD' | 'INSTRUCTOR'
  label: string
  status: 'ACTIVE' | 'ARCHIVED'
  categories: string[]
  indicative_capacity: number | null
  linked_user_id: string | null
  out_right_now: boolean
  future_lessons: number
  ongoing_unavailability: {
    id: string
    reason: string
    starts_at: string
    ends_at: string | null
  } | null
}

export interface ImpactedLesson {
  id: string
  starts_at: string
  ends_at: string
  students: number
}

export interface PersonRow {
  id: string
  last_name: string
  first_names: string
  date_of_birth: string | null
  phone: string | null
  email: string | null
  neph: string | null
  status: 'ACTIVE' | 'ARCHIVED'
  enrollment: {
    id: string
    objective: string
    offroad_target_date: string | null
    onroad_target_date: string | null
    consumed_hours: number
    total_hours: number
    upcoming_lessons: number
    funding_status: string
    funder_label: string
  } | null
  health: Health
}

export interface Requirement {
  id: string
  label: string
  set: string
  mandatory: boolean
  status: 'NOT_VALIDATED' | 'VALIDATED' | 'NOT_APPLICABLE'
  validated_by: string
  validated_at: string | null
  comment: string
  na_reason: string
  valid_until: string | null
  expired: boolean
  can_validate: boolean
}

export interface RequirementSet {
  title: string
  items: Requirement[]
  missing: number
  complete: boolean
}

export interface RequirementTemplate {
  id: string
  label: string
  set: string
  mandatory: boolean
  instructor_may_validate: boolean
  validity_months: number | null
  active: boolean
}

export interface WeekLine {
  week: string
  range: string
  be: number
  isoles: number
  ensembles: number
  students?: string[]
}

export interface Communication {
  id: string
  channel: 'EMAIL' | 'WHATSAPP'
  kind: string
  recipient_label: string
  recipient_address: string
  subject: string
  body: string
  status: 'PREPARED' | 'SENT' | 'SIMULATED' | 'FAILED'
  whatsapp_link?: string
  created_at: string
  sent_at: string | null
}

export interface Task {
  kind: string
  title: string
  detail: string
  severity: string
  due: string | null
  resource_id?: string
  unavailability_id?: string
  count: number
}

export interface AbsenceReason {
  id: string
  label: string
  active: boolean
}

export interface ExamBookingRow {
  id: string
  enrollment_id: string
  student_name: string
  objective: string
  test_kind: 'OFFROAD' | 'ONROAD'
  units: number
  status: string
  override_note: string
  booked_by: string
  booked_at: string
  unrecorded: boolean
  consumed_hours: number
  total_hours: number
}

export interface ExamCandidate {
  enrollment_id: string
  student_name: string
  objective: string
  consumed_hours: number
  total_hours: number
  projected_hours: number | null
  threshold_hours: number
  target_date: string | null
  days_left: number | null
  missing_requirements: string[]
  offroad_passed_at: string | null
  offroad_expires_at: string | null
  presentations: number
}

export interface ExamSessionDetail {
  session: ExamSessionRow
  resource_labels: string[]
  credits: { allowance: number | null; committed: number; spent: number; forfeited: number; remaining: number | null }
  bookings: ExamBookingRow[]
  proposed: ExamCandidate[]
  active_total: number
  proposed_total: number
}

export interface FundingView {
  id: string
  status: 'DRAFT' | 'SUBMITTED' | 'APPROVED' | 'SETTLED' | 'REJECTED'
  funder_kind_id: string | null
  funder_label: string
  self_funded: boolean
  payer_id: string | null
  payer: Payer | null
  transitions:
    | { from: string; to: string; reason: string; author: string; at: string }[]
    | null
}

export interface Payer {
  id: string
  label: string
  contact_name: string
  contact_email: string
  contact_phone: string
  active: boolean
}

export interface FunderKind {
  id: string
  label: string
  self_funded: boolean
  active: boolean
}

export interface Health {
  color: 'green' | 'amber' | 'red'
  reasons: string[]
}

export interface HistoryEvent {
  kind: string
  occurred_at: string
  author: string
}

export interface StudentLesson {
  starts_at: string
  ends_at: string
  minutes: number
  cancelled: boolean
  upcoming: boolean
  value: string
  resources: string[]
}

export interface Objective {
  id: string
  label: string
  hours_before_offroad: number
  total_hours: number
  active: boolean
}

export interface KindRow {
  id: string
  label: string
  requires_vehicle: boolean
  active: boolean
}

export interface DurationRow {
  id: string
  minutes: number
  active: boolean
}

export interface CategoryRow {
  id: string
  code: string
  active: boolean
}

export interface OpeningHourRow {
  weekday: number
  start: string
  end: string
}

export interface AttendanceLine {
  assignment_id: string
  enrollment_id: string
  student_name: string
  value: '' | 'PRESENT' | 'EXCUSED' | 'UNEXCUSED'
  reason: string
}

export interface LessonAttendance {
  lesson_id: string
  starts_at: string
  ends_at: string
  students: AttendanceLine[]
}

export interface UnrecordedLesson {
  lesson_id: string
  starts_at: string
  ends_at: string
  students: number
  recorded: number
  instructors: string[]
  vehicles: string[]
}

export interface ResourceDetail {
  id: string
  kind: string
  label: string
  status: string
  unavailabilities: {
    id: string
    reason: string
    starts_at: string
    ends_at: string | null
    status: string
    restored_note: string | null
  }[]
  upcoming_lessons: ImpactedLesson[]
}

export interface ExamPlace {
  id: string
  label: string
  travel_minutes: number
  active: boolean
}

export interface ExamSessionRow {
  id: string
  place_id: string
  place_label: string
  travel_minutes: number
  starts_at: string
  ends_at: string
  credit_allowance: number | null
  cancelled: boolean
  cancel_reason: string
  past: boolean
  resources: string[]
}

export interface SheetLine {
  date: string
  starts_at: string
  ends_at: string
  minutes: number
  student_name: string
  value: string
}

export interface EnrollmentDetail {
  id: string
  student_name: string
  objective: string
  life_status: string
  hours_before_offroad: number
  total_hours: number
  offroad_target_date: string | null
  onroad_target_date: string | null
  hours: {
    attended: number
    excused: number
    unexcused: number
    consumed: number
    projected_offroad: number | null
    projected_onroad: number | null
  }
  offroad_passed_at: string | null
  offroad_expires_at: string | null
  alerts: {
    kind: string
    severity: string
    target: string
    gap_hours: number
    days_left: number
    message: string
  }[]
}

export interface GapLine {
  enrollment_id: string
  student_name: string
  objective: string
  target: string
  target_date: string
  days_left: number
  gap_hours: number
  projected_hours: number
  threshold_hours: number
}

export interface SuggestedStudent {
  enrollment_id: string
  student_name: string
  objective: string
  gap_hours: number
  target: string
  target_date: string | null
  days_left: number | null
  projected_hours: number | null
  threshold_hours: number | null
}

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body !== undefined ? { 'Content-Type': 'application/json' } : {},
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  if (res.status === 204) return undefined as T
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new ApiError(res.status, data)
  return data as T
}

export const api = {
  login: (login: string, secret: string) =>
    call<UserContext>('POST', '/api/auth/login', { login, secret }),

  logout: () => call<void>('POST', '/api/auth/logout'),

  me: () => call<UserContext>('GET', '/api/me'),

  planning: (from: string, to: string) =>
    call<PlanningView>('GET', `/api/planning?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`),

  openSession: (from: string, to: string) =>
    call<EditSession>('POST', '/api/edit-sessions', {
      scope_starts_at: from,
      scope_ends_at: to,
    }),

  pushDraft: (sessionId: string, ops: Operation[]) =>
    call<SessionView>('PUT', `/api/edit-sessions/${sessionId}/operations`, ops),

  getSession: (sessionId: string) => call<SessionView>('GET', `/api/edit-sessions/${sessionId}`),

  forceRelease: (sessionId: string) =>
    call<void>('POST', `/api/edit-sessions/${sessionId}/force-release`),

  save: (sessionId: string) =>
    call<{ batch_id: string; applied: Operation[]; overridden: Conflict[] }>(
      'POST',
      `/api/edit-sessions/${sessionId}/save`,
    ),

  discard: (sessionId: string) => call<void>('POST', `/api/edit-sessions/${sessionId}/discard`),

  // F-15 / F-16
  resources: (kind?: string) =>
    call<ResourceRow[]>('GET', `/api/resources${kind ? `?kind=${kind}` : ''}`),
  createResource: (body: {
    kind: string
    label: string
    categories?: string[]
    indicative_capacity?: number
  }) => call<{ id: string }>('POST', '/api/resources', body),
  patchResource: (id: string, body: Record<string, unknown>) =>
    call<void>('PATCH', `/api/resources/${id}`, body),
  resourceDetail: (id: string) => call<ResourceDetail>('GET', `/api/resources/${id}`),
  declareUnavailability: (id: string, body: { reason: string; starts_at: string; ends_at?: string }) =>
    call<{ impacted_lessons: ImpactedLesson[] }>('POST', `/api/resources/${id}/unavailabilities`, body),

  // F-02 / F-04
  persons: (search?: string) =>
    call<PersonRow[]>('GET', `/api/persons${search ? `?search=${encodeURIComponent(search)}` : ''}`),
  createPerson: (body: Record<string, unknown>) => call<{ id: string }>('POST', '/api/persons', body),
  patchPerson: (id: string, body: Record<string, unknown>) =>
    call<void>('PATCH', `/api/persons/${id}`, body),
  objectives: () => call<Objective[]>('GET', '/api/objectives'),
  createObjective: (body: Record<string, unknown>) =>
    call<{ id: string }>('POST', '/api/objectives', body),
  patchObjective: (id: string, body: Record<string, unknown>) =>
    call<void>('PATCH', `/api/objectives/${id}`, body),
  createEnrollment: (body: Record<string, unknown>) =>
    call<{ id: string }>('POST', '/api/enrollments', body),

  // F-01
  lessonKinds: () => call<KindRow[]>('GET', '/api/settings/lesson-kinds'),
  createLessonKind: (body: { label: string; requires_vehicle: boolean }) =>
    call<KindRow>('POST', '/api/settings/lesson-kinds', body),
  patchLessonKind: (id: string, body: Record<string, unknown>) =>
    call<void>('PATCH', `/api/settings/lesson-kinds/${id}`, body),
  durations: () => call<DurationRow[]>('GET', '/api/settings/lesson-durations'),
  createDuration: (minutes: number) =>
    call<DurationRow>('POST', '/api/settings/lesson-durations', { minutes }),
  patchDuration: (id: string, body: Record<string, unknown>) =>
    call<void>('PATCH', `/api/settings/lesson-durations/${id}`, body),
  categories: () => call<CategoryRow[]>('GET', '/api/settings/licence-categories'),
  createCategory: (code: string) =>
    call<CategoryRow>('POST', '/api/settings/licence-categories', { code }),
  patchCategory: (id: string, body: Record<string, unknown>) =>
    call<void>('PATCH', `/api/settings/licence-categories/${id}`, body),
  openingHours: () => call<OpeningHourRow[]>('GET', '/api/settings/opening-hours'),
  replaceOpeningHours: (hours: OpeningHourRow[]) =>
    call<void>('PUT', '/api/settings/opening-hours', hours),

  // F-12
  currentLesson: () => call<LessonAttendance | undefined>('GET', '/api/attendance/current'),
  unrecordedLessons: () => call<UnrecordedLesson[]>('GET', '/api/attendance/unrecorded'),
  lessonAttendance: (lessonId: string) =>
    call<LessonAttendance>('GET', `/api/lessons/${lessonId}/attendance`),
  recordAttendance: (lessonId: string, lines: { enrollment_id: string; value: string; reason?: string }[]) =>
    call<LessonAttendance>('PUT', `/api/lessons/${lessonId}/attendance`, lines),
  attendanceSheet: (enrollmentId: string) =>
    call<SheetLine[]>('GET', `/api/attendance/sheet?enrollment_id=${enrollmentId}`),

  // F-06
  examPlaces: () => call<ExamPlace[]>('GET', '/api/exam-places'),
  createExamPlace: (body: { label: string; travel_minutes: number }) =>
    call<ExamPlace>('POST', '/api/exam-places', body),
  patchExamPlace: (id: string, body: Record<string, unknown>) =>
    call<void>('PATCH', `/api/exam-places/${id}`, body),
  examSessions: () => call<ExamSessionRow[]>('GET', '/api/exam-sessions'),
  createExamSession: (body: {
    place_id: string
    starts_at: string
    ends_at: string
    credit_allowance?: number
    resources: string[]
  }) => call<{ id: string }>('POST', '/api/exam-sessions', body),
  cancelExamSession: (id: string, reason: string) =>
    call<void>('POST', `/api/exam-sessions/${id}/cancel`, { reason }),

  personHistory: (id: string) => call<HistoryEvent[]>('GET', `/api/persons/${id}/history`),
  requirements: (enrollmentId: string) =>
    call<{ entry: RequirementSet; exam: RequirementSet }>(
      'GET',
      `/api/enrollments/${enrollmentId}/requirements`,
    ),
  actRequirement: (id: string, action: 'validate' | 'unvalidate' | 'not-applicable', body?: Record<string, string>) =>
    call<void>('POST', `/api/requirements/${id}/${action}`, body ?? {}),
  requirementTemplates: (objectiveId: string) =>
    call<RequirementTemplate[]>('GET', `/api/objectives/${objectiveId}/requirement-templates`),
  createRequirementTemplate: (objectiveId: string, body: Record<string, unknown>) =>
    call<{ id: string }>('POST', `/api/objectives/${objectiveId}/requirement-templates`, body),
  patchRequirementTemplate: (id: string, body: Record<string, unknown>) =>
    call<void>('PATCH', `/api/requirement-templates/${id}`, body),
  studentLessons: (enrollmentId: string) =>
    call<StudentLesson[]>('GET', `/api/enrollments/${enrollmentId}/lessons`),

  // F-36 / tasks
  tasks: () => call<Task[]>('GET', '/api/tasks'),
  restoreUnavailability: (id: string, note?: string) =>
    call<{ kept_lessons: ImpactedLesson[] }>(
      'POST',
      `/api/unavailabilities/${id}/restore`,
      note ? { note } : undefined,
    ),

  // F-33 — calibrated on the centre's real files: weeks × groups
  generateSeatRequest: (month: string, lines?: WeekLine[]) =>
    call<{ lines: WeekLine[] }>('POST', `/api/seat-requests/${month}/generate`, lines ? { lines } : {}),

  seatRequestSuggestion: (month: string) =>
    call<{
      deadline: string
      deadline_passed: boolean
      suggested: WeekLine[]
      generated_at: string | null
      generated_lines: WeekLine[]
    }>('GET', `/api/seat-requests/${month}/suggestion`),

  // F-20
  dashboard: () =>
    call<{
      active_students: number
      lessons_this_week: number
      hours_this_month: number
      upcoming_exams: number
      committed_units: number
      gap_count: number
      unrecorded_count: number
      weekly: { week_start: string; lessons: number; hours: number }[]
      funding_breakdown: { label: string; count: number }[]
      permit_breakdown: { label: string; count: number }[]
    }>('GET', '/api/dashboard'),

  // F-35 documents / e-photo
  requestEphoto: (personId: string) =>
    call<{ token: string; path: string }>('POST', `/api/persons/${personId}/ephoto-request`),
  personDocuments: (personId: string) =>
    call<
      { id: string; kind: string; filename: string; ants_code: string; via: string; uploaded_at: string }[]
    >('GET', `/api/persons/${personId}/documents`),

  // Communications
  convokeSession: (sessionId: string, bookingId?: string) =>
    call<{ communications: Communication[]; smtp_configured: boolean }>(
      'POST',
      `/api/exam-sessions/${sessionId}/convocations`,
      bookingId ? { booking_id: bookingId } : {},
    ),
  personCommunications: (personId: string) =>
    call<Communication[]>('GET', `/api/persons/${personId}/communications`),
  sendFreeMessage: (personId: string, body: { channel: string; subject?: string; body: string }) =>
    call<Communication>('POST', `/api/persons/${personId}/communications`, body),
  markCommunicationSent: (id: string) => call<void>('POST', `/api/communications/${id}/mark-sent`),
  communicationTemplates: () =>
    call<{ kind: string; subject: string; body: string }[]>('GET', '/api/communication-templates'),
  putCommunicationTemplate: (kind: string, body: { subject: string; body: string }) =>
    call<void>('PUT', `/api/communication-templates/${kind}`, body),

  // F-12 reasons
  absenceReasons: () => call<AbsenceReason[]>('GET', '/api/absence-reasons'),
  createAbsenceReason: (label: string) =>
    call<AbsenceReason>('POST', '/api/absence-reasons', { label }),
  patchAbsenceReason: (id: string, body: Record<string, unknown>) =>
    call<void>('PATCH', `/api/absence-reasons/${id}`, body),

  // F-07.1
  examSessionDetail: (id: string) => call<ExamSessionDetail>('GET', `/api/exam-sessions/${id}`),
  enterExamResult: (bookingId: string, status: 'PASSED' | 'FAILED' | 'ABSENT') =>
    call<void>('POST', `/api/exam-bookings/${bookingId}/result`, { status }),
  bookExam: (sessionId: string, enrollmentId: string, testKind: 'OFFROAD' | 'ONROAD') =>
    call<{ id: string; override_note: string | null }>('POST', `/api/exam-sessions/${sessionId}/bookings`, {
      enrollment_id: enrollmentId,
      test_kind: testKind,
    }),
  withdrawBooking: (id: string) => call<void>('POST', `/api/exam-bookings/${id}/withdraw`),

  // F-05
  funding: (enrollmentId: string) => call<FundingView>('GET', `/api/enrollments/${enrollmentId}/funding`),
  fundingTransition: (enrollmentId: string, status: string, reason?: string) =>
    call<void>('POST', `/api/enrollments/${enrollmentId}/funding/transition`, {
      status,
      ...(reason ? { reason } : {}),
    }),
  patchFunding: (enrollmentId: string, funderKindId: string | null) =>
    call<void>('PATCH', `/api/enrollments/${enrollmentId}/funding`, { funder_kind_id: funderKindId }),
  funderKinds: () => call<FunderKind[]>('GET', '/api/funder-kinds'),
  payers: () => call<Payer[]>('GET', '/api/payers'),
  createPayer: (body: Record<string, string>) => call<{ id: string }>('POST', '/api/payers', body),
  setPayer: (enrollmentId: string, payerId: string | null) =>
    call<void>('PUT', `/api/enrollments/${enrollmentId}/payer`, { payer_id: payerId }),

  // F-03 direct placement (A-38)
  placeDirect: (lessonId: string, enrollmentId: string) =>
    call<{ lesson: Lesson; conflicts: Conflict[]; communication?: Communication }>(
      'POST',
      `/api/lessons/${lessonId}/assignments`,
      { enrollment_id: enrollmentId },
    ),
  removeDirect: (lessonId: string, enrollmentId: string) =>
    call<{ lesson: Lesson; conflicts: Conflict[] }>(
      'DELETE',
      `/api/lessons/${lessonId}/assignments/${enrollmentId}`,
    ),

  // F-13 / F-10
  enrollmentDetail: (id: string) => call<EnrollmentDetail>('GET', `/api/enrollments/${id}`),
  gaps: () => call<GapLine[]>('GET', '/api/enrollments/gaps'),
  suggestedStudents: (lessonId: string) =>
    call<SuggestedStudent[]>('GET', `/api/lessons/${lessonId}/suggested-students`),
}
