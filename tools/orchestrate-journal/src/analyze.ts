import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import {
  attributeEntries,
  childStats,
  findDispatch,
  findParentTranscript,
  parentStats,
  readTranscript,
  shellKinds,
  type ChildStats,
  type Dispatch,
  type ParentStats,
  type Transcript,
} from './cursor.ts';
import { JournalError, readJournal, type Journal, type JournalEvent } from './journal.ts';
import { attemptIdPattern } from './schema.ts';

// Data the analysis could not obtain from transcripts, briefs, reports, or the
// hook; journal contract violations throw instead.
export interface Gap {
  task_id: string;
  attempt_id: string | null;
  message: string;
}

export interface AttemptRecord {
  attempt_id: string;
  work_item_id: string;
  n: number;
  role: string | null;
  child_type: string | null;
  requested_model: string | null;
  background: boolean | null;
  // Source of child_type, requested_model, and background: the hook's
  // attempt.finished, or the parent's Task call when the hook missed the attempt.
  dispatch_source: 'hook' | 'parent_transcript' | null;
  duration_ms: number | null;
  // Background durations run from launch to the child transcript's last write.
  duration_estimated: boolean;
  harness_status: string | null;
  child_status: string | null;
  decision: string | null;
  reason: string | null;
  parent_model: string | null;
  skill_revision: string;
  native_child_id: string | null;
  brief_path: string;
  brief_bytes: number | null;
  report_path: string;
  report_bytes: number | null;
  child_transcript_path: string | null;
  child: ChildStats | null;
}

export interface WorkItemRecord {
  work_item_id: string;
  roles: string[];
  attempts: number;
  corrections: number;
  first_attempt_accepted: boolean | null;
  latest_decision: string | null;
}

export interface SessionRecord {
  index: number;
  conversation_id: string | null;
  harness: string | null;
  model: string | null;
  effort: string | null;
  skill_revision: string;
  outcome: string | null;
  criteria_unmet: string[];
  open_work_items: string[];
  parent_transcript_path: string | null;
}

export interface ConversationRecord {
  conversation_id: string;
  transcript_path: string;
  // 'task' when the stats cover only this task's part of a conversation shared
  // with other tasks; 'conversation' when the part could not be told apart, so
  // they cover all of it.
  scope: 'task' | 'conversation';
  stats: ParentStats;
}

export interface TaskRecord {
  task_id: string;
  journal_path: string;
  task_dir: string;
  title: string | null;
  state: 'completed' | 'blocked' | 'open';
  criteria_changes: number;
  sessions: SessionRecord[];
  conversations: ConversationRecord[];
  work_items: WorkItemRecord[];
  attempts: AttemptRecord[];
  bytes: { briefs: number; reports: number; journal_parent: number; journal_hook: number };
}

export interface GroupStats {
  attempts: number;
  first_attempts_reviewed: number;
  first_attempts_accepted: number;
  corrections: number;
  reasons: Record<string, number>;
  durations_known: number;
  durations_estimated: number;
  duration_ms_total: number;
  duration_ms_median: number | null;
  child_tool_calls_known: number;
  child_tool_calls_total: number;
  child_tool_calls_median: number | null;
  child_follow_ups: number;
  brief_bytes: number;
  report_bytes: number;
  non_success: Record<string, number>;
}

export interface ParentGroupStats {
  sessions: number;
  conversations_with_transcript: number;
  tool_calls: number;
  dispatches: number;
  user_messages: number;
  notifications: number;
  shell: ParentStats['shell'];
  source_edits: number;
  source_edit_bytes: number;
  source_reads: number;
}

export type Dimension = 'role' | 'requested_model' | 'skill_revision' | 'parent_model';
export const dimensions: readonly Dimension[] = ['role', 'requested_model', 'skill_revision', 'parent_model'];

export interface Analysis {
  tasks: TaskRecord[];
  aggregates: Record<Dimension, Record<string, GroupStats>>;
  parent_by_skill_revision: Record<string, ParentGroupStats>;
  gaps: Gap[];
}

export interface AnalyzeOptions {
  cursor_dir: string;
}

export class InvalidJournalsError extends Error {
  constructor(errors: string[]) {
    super(`invalid journals\n${errors.join('\n')}`);
  }
}

const str = (value: unknown): string | null => (typeof value === 'string' ? value : null);
const strings = (value: unknown): string[] =>
  Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : [];

function fileBytes(path: string): number | null {
  try {
    return statSync(path).size;
  } catch {
    return null;
  }
}

// Parents write their own model ID from memory, as claude-opus-5.5 or claude-opus-5-5.
export const modelLabel = (model: string | null): string | null =>
  model === null ? null : model.toLowerCase().replace(/(\d)\.(\d)/g, '$1-$2');

export function revisionLabel(revision: unknown): string {
  if (revision === null) return 'unknown';
  const { commit, dirty } = revision as { commit: string; dirty: boolean };
  return `${commit.slice(0, 12)}${dirty ? '+dirty' : ''}`;
}

function briefRole(path: string): string | null {
  try {
    const match = /^Role: (.+)$/m.exec(readFileSync(path, 'utf8'));
    return match?.[1]?.trim() ?? null;
  } catch {
    return null;
  }
}

function attemptIdsOnDisk(taskDir: string): string[] {
  const briefs = join(taskDir, 'briefs');
  if (!existsSync(briefs)) return [];
  return readdirSync(briefs)
    .filter((name) => name.endsWith('.md'))
    .map((name) => name.slice(0, -3))
    .filter((id) => attemptIdPattern.test(id));
}

interface TaskAnalysis {
  task: TaskRecord;
  // Parent transcript path by conversation ID, in session order.
  transcripts: Map<string, string>;
}

type TranscriptReader = (path: string) => Transcript;

function transcriptCache(): TranscriptReader {
  const cache = new Map<string, Transcript>();
  return (path) => {
    const cached = cache.get(path) ?? readTranscript(path);
    cache.set(path, cached);
    return cached;
  };
}

function analyzeTask(journal: Journal, options: AnalyzeOptions, readParent: TranscriptReader, gaps: Gap[]): TaskAnalysis {
  const taskId = journal.task_id;
  const gap = (message: string, attemptId: string | null = null) =>
    gaps.push({ task_id: taskId, attempt_id: attemptId, message });
  const { events } = journal;
  const ofType = (type: string) => events.filter((event) => event.type === type);

  const sessions: SessionRecord[] = journal.sessions.map((start, index) => {
    const finish = ofType('session.finished').find((event) => event.session === index);
    return {
      index,
      conversation_id: str(start.data.conversation_id),
      harness: str(start.data.harness),
      model: modelLabel(str(start.data.model)),
      effort: str(start.data.effort),
      skill_revision: revisionLabel(start.data.skill_revision),
      outcome: finish === undefined ? null : str(finish.data.outcome),
      criteria_unmet: strings(finish?.data.criteria_unmet),
      open_work_items: strings(finish?.data.open_work_items),
      parent_transcript_path: null,
    };
  });
  if (sessions.length === 0) gap('no session.started');

  const transcripts = new Map<string, string>();
  for (const session of sessions) {
    if (session.harness === 'codex') {
      gap(`session ${session.index + 1}: Codex rollouts are not analyzed yet`);
      continue;
    }
    if (session.harness !== 'cursor') {
      gap(`session ${session.index + 1}: unknown harness ${JSON.stringify(session.harness)}; parent transcript not read`);
      continue;
    }
    if (session.conversation_id === null) {
      gap(`session ${session.index + 1}: no conversation_id; parent transcript not read`);
      continue;
    }
    const existing = transcripts.get(session.conversation_id);
    if (existing !== undefined) {
      session.parent_transcript_path = existing;
      continue;
    }
    const path = findParentTranscript(options.cursor_dir, session.conversation_id);
    if (path === null) {
      gap(`session ${session.index + 1}: parent transcript not found for conversation ${session.conversation_id}`);
      continue;
    }
    session.parent_transcript_path = path;
    transcripts.set(session.conversation_id, path);
  }

  const attemptEvents = new Map<string, JournalEvent[]>();
  for (const id of attemptIdsOnDisk(journal.task_dir)) attemptEvents.set(id, []);
  for (const event of events) {
    if (event.attempt_id === null || !attemptIdPattern.test(event.attempt_id)) continue;
    if (event.type !== 'attempt.finished' && event.type !== 'attempt.reviewed') continue;
    attemptEvents.set(event.attempt_id, [...(attemptEvents.get(event.attempt_id) ?? []), event]);
  }

  const attempts: AttemptRecord[] = [];
  for (const [attemptId, attemptEventList] of attemptEvents) {
    const match = attemptIdPattern.exec(attemptId);
    if (match === null) continue;
    const attemptGap = (message: string) => gap(message, attemptId);
    const finishes = attemptEventList.filter((event) => event.type === 'attempt.finished');
    if (finishes.length > 1) attemptGap(`${finishes.length} attempt.finished events; the latest is used`);
    const finish = finishes.at(-1)?.data ?? null;
    const review = attemptEventList.find((event) => event.type === 'attempt.reviewed')?.data ?? null;
    // An attempt known only from its brief was dispatched in the latest session.
    const sessionIndex = attemptEventList.length === 0 ? sessions.length - 1 : (attemptEventList[0]?.session ?? null);
    const session = sessionIndex === null ? undefined : sessions[sessionIndex];

    const briefPath = join(journal.task_dir, 'briefs', `${attemptId}.md`);
    const reportPath = join(journal.task_dir, 'reports', `${attemptId}.md`);
    const briefBytes = fileBytes(briefPath);
    const reportBytes = fileBytes(reportPath);
    if (briefBytes === null) attemptGap('brief missing');
    if (reportBytes === null) attemptGap('report missing');

    const hookRole = str(finish?.role);
    const diskRole = briefBytes === null ? null : briefRole(briefPath);
    if (hookRole !== null && diskRole !== null && hookRole !== diskRole) {
      attemptGap(`hook role ${hookRole} differs from brief role ${diskRole}`);
    }
    let dispatch: Dispatch | null = null;
    if (finish === null) {
      for (const path of transcripts.values()) dispatch = findDispatch(readParent(path).entries, taskId, attemptId) ?? dispatch;
    }
    if (finish === null && session?.harness === 'cursor') {
      attemptGap(
        dispatch === null
          ? 'no attempt.finished; the hook did not record it'
          : 'no attempt.finished; dispatch fields recovered from the parent transcript',
      );
    }
    if (review === null) attemptGap('not reviewed');

    const transcriptPath = str(finish?.child_transcript_path);
    let child: ChildStats | null = null;
    let lastWrite: number | null = null;
    if (transcriptPath !== null) {
      if (existsSync(transcriptPath)) {
        const transcript = readTranscript(transcriptPath);
        if (transcript.malformed_lines > 0) attemptGap(`child transcript: ${transcript.malformed_lines} malformed lines`);
        child = childStats(transcript);
        lastWrite = statSync(transcriptPath).mtimeMs;
      } else {
        attemptGap(`child transcript missing: ${transcriptPath}`);
      }
    }

    const background = typeof finish?.background === 'boolean' ? finish.background : (dispatch?.background ?? null);
    let duration = typeof finish?.duration_ms === 'number' ? finish.duration_ms : null;
    let durationEstimated = false;
    if (duration === null && background === true && lastWrite !== null) {
      const launched = Date.parse(str(finish?.started_at) ?? '');
      if (!Number.isNaN(launched) && lastWrite > launched) {
        duration = Math.round(lastWrite - launched);
        durationEstimated = true;
      }
    }

    attempts.push({
      attempt_id: attemptId,
      work_item_id: match[1] ?? attemptId,
      n: Number(match[2]),
      role: hookRole ?? diskRole,
      child_type: finish === null ? (dispatch?.child_type ?? null) : str(finish.child_type),
      requested_model: finish === null ? (dispatch?.requested_model ?? null) : str(finish.requested_model),
      background,
      dispatch_source: finish !== null ? 'hook' : dispatch !== null ? 'parent_transcript' : null,
      duration_ms: duration,
      duration_estimated: durationEstimated,
      harness_status: str(finish?.harness_status),
      child_status: str(review?.child_status),
      decision: str(review?.decision),
      reason: str(review?.reason),
      parent_model: session?.model ?? null,
      skill_revision: session?.skill_revision ?? 'unknown',
      native_child_id: str(finish?.native_child_id),
      brief_path: briefPath,
      brief_bytes: briefBytes,
      report_path: reportPath,
      report_bytes: reportBytes,
      child_transcript_path: transcriptPath,
      child,
    });
  }
  const firstLine = new Map<string, number>();
  for (const attempt of attempts) {
    const line = attemptEvents.get(attempt.attempt_id)?.[0]?.line ?? Number.POSITIVE_INFINITY;
    firstLine.set(attempt.work_item_id, Math.min(line, firstLine.get(attempt.work_item_id) ?? line));
  }
  const order = (attempt: AttemptRecord) => firstLine.get(attempt.work_item_id) ?? Number.POSITIVE_INFINITY;
  attempts.sort((a, b) => order(a) - order(b) || a.work_item_id.localeCompare(b.work_item_id) || a.n - b.n);

  const workItems = new Map<string, AttemptRecord[]>();
  for (const attempt of attempts) workItems.set(attempt.work_item_id, [...(workItems.get(attempt.work_item_id) ?? []), attempt]);
  const workItemRecords: WorkItemRecord[] = [...workItems].map(([workItemId, items]) => {
    items.forEach((attempt, index) => {
      if (attempt.n !== index + 1) gap(`work item ${workItemId}: attempt numbers are not consecutive from 1`, attempt.attempt_id);
    });
    const first = items.find((attempt) => attempt.n === 1);
    return {
      work_item_id: workItemId,
      roles: [...new Set(items.map((attempt) => attempt.role ?? 'unknown'))],
      attempts: items.length,
      corrections: items.filter((attempt) => attempt.decision === 'correction_required').length,
      first_attempt_accepted: first?.decision == null ? null : first.decision === 'accepted',
      latest_decision: items.at(-1)?.decision ?? null,
    };
  });

  // A blocked task can be completed by a later session, so the latest end event is its state.
  const lastTaskEnd = events.filter((event) => event.type === 'task.completed' || event.type === 'task.blocked').at(-1);

  const sum = (values: (number | null)[]) => values.reduce<number>((total, value) => total + (value ?? 0), 0);
  const journalBytes = (writer: string) =>
    sum(journal.all_events.filter((event) => event.writer === writer).map((event) => event.bytes));

  const task: TaskRecord = {
    task_id: taskId,
    journal_path: journal.path,
    task_dir: journal.task_dir,
    title: str(ofType('task.started').at(-1)?.data.title),
    state: lastTaskEnd === undefined ? 'open' : lastTaskEnd.type === 'task.completed' ? 'completed' : 'blocked',
    criteria_changes: ofType('task.criteria_changed').length,
    sessions,
    conversations: [],
    work_items: workItemRecords,
    attempts,
    bytes: {
      briefs: sum(attempts.map((attempt) => attempt.brief_bytes)),
      reports: sum(attempts.map((attempt) => attempt.report_bytes)),
      journal_parent: journalBytes('parent'),
      journal_hook: journalBytes('hook'),
    },
  };
  return { task, transcripts };
}

// Reads each parent transcript once and gives every task that shares it the
// stats of its own part.
function addConversations(analyses: TaskAnalysis[], readParent: TranscriptReader, gaps: Gap[]): void {
  const owners = new Map<string, { path: string; tasks: TaskRecord[] }>();
  for (const { task, transcripts } of analyses) {
    for (const [conversationId, path] of transcripts) {
      const owner = owners.get(conversationId) ?? { path, tasks: [] };
      owner.tasks.push(task);
      owners.set(conversationId, owner);
    }
  }
  for (const [conversationId, { path, tasks }] of owners) {
    const transcript = readParent(path);
    if (transcript.malformed_lines > 0) {
      for (const task of tasks) {
        gaps.push({ task_id: task.task_id, attempt_id: null, message: `parent transcript ${path}: ${transcript.malformed_lines} malformed lines` });
      }
    }
    const parts = attributeEntries(
      transcript.entries,
      tasks.map((task) => task.task_id),
    );
    const scope = parts === null ? 'conversation' : 'task';
    for (const task of tasks) {
      if (parts === null && tasks.length > 1) {
        const others = tasks.filter((other) => other !== task).map((other) => other.task_id);
        gaps.push({
          task_id: task.task_id,
          attempt_id: null,
          message: `conversation ${conversationId} also serves ${others.join(', ')}, but its tool calls do not name every task directory; its parent stats cover the whole conversation for each task`,
        });
      }
      const entries = parts?.get(task.task_id) ?? transcript.entries;
      task.conversations.push({ conversation_id: conversationId, transcript_path: path, scope, stats: parentStats(entries, task.task_dir) });
    }
  }
}

function median(values: number[]): number | null {
  if (values.length === 0) return null;
  const sorted = [...values].sort((a, b) => a - b);
  const middle = Math.floor(sorted.length / 2);
  return sorted.length % 2 === 1 ? (sorted[middle] ?? null) : ((sorted[middle - 1] ?? 0) + (sorted[middle] ?? 0)) / 2;
}

export function groupStats(attempts: AttemptRecord[]): GroupStats {
  const firsts = attempts.filter((attempt) => attempt.n === 1 && attempt.decision !== null);
  const durations = attempts.flatMap((attempt) => (attempt.duration_ms === null ? [] : [attempt.duration_ms]));
  const estimated = attempts.filter((attempt) => attempt.duration_ms !== null && attempt.duration_estimated).length;
  const toolCalls = attempts.flatMap((attempt) => (attempt.child === null ? [] : [attempt.child.tool_calls]));
  const reasons: Record<string, number> = {};
  const nonSuccess: Record<string, number> = {};
  for (const attempt of attempts) {
    if (attempt.reason !== null) reasons[attempt.reason] = (reasons[attempt.reason] ?? 0) + 1;
    for (const status of [
      attempt.harness_status === null || attempt.harness_status === 'success' ? null : `harness:${attempt.harness_status}`,
      attempt.child_status === null || attempt.child_status === 'complete' ? null : `child:${attempt.child_status}`,
    ]) {
      if (status !== null) nonSuccess[status] = (nonSuccess[status] ?? 0) + 1;
    }
  }
  return {
    attempts: attempts.length,
    first_attempts_reviewed: firsts.length,
    first_attempts_accepted: firsts.filter((attempt) => attempt.decision === 'accepted').length,
    corrections: attempts.filter((attempt) => attempt.decision === 'correction_required').length,
    reasons,
    durations_known: durations.length,
    durations_estimated: estimated,
    duration_ms_total: durations.reduce((total, value) => total + value, 0),
    duration_ms_median: median(durations),
    child_tool_calls_known: toolCalls.length,
    child_tool_calls_total: toolCalls.reduce((total, value) => total + value, 0),
    child_tool_calls_median: median(toolCalls),
    child_follow_ups: attempts.filter((attempt) => (attempt.child?.follow_ups ?? 0) > 0).length,
    brief_bytes: attempts.reduce((total, attempt) => total + (attempt.brief_bytes ?? 0), 0),
    report_bytes: attempts.reduce((total, attempt) => total + (attempt.report_bytes ?? 0), 0),
    non_success: nonSuccess,
  };
}

function aggregate(tasks: TaskRecord[]): Analysis['aggregates'] {
  const attempts = tasks.flatMap((task) => task.attempts);
  const result = {} as Analysis['aggregates'];
  for (const dimension of dimensions) {
    const groups = new Map<string, AttemptRecord[]>();
    for (const attempt of attempts) {
      const key = attempt[dimension] ?? 'unknown';
      groups.set(key, [...(groups.get(key) ?? []), attempt]);
    }
    result[dimension] = Object.fromEntries([...groups].sort(([a], [b]) => a.localeCompare(b)).map(([key, group]) => [key, groupStats(group)]));
  }
  return result;
}

function aggregateParents(tasks: TaskRecord[]): Analysis['parent_by_skill_revision'] {
  const groups: Record<string, ParentGroupStats> = {};
  const group = (revision: string) =>
    (groups[revision] ??= {
      sessions: 0,
      conversations_with_transcript: 0,
      tool_calls: 0,
      dispatches: 0,
      user_messages: 0,
      notifications: 0,
      shell: Object.fromEntries(shellKinds.map((kind) => [kind, 0])) as ParentStats['shell'],
      source_edits: 0,
      source_edit_bytes: 0,
      source_reads: 0,
    });
  // Task-scoped parts of a conversation are disjoint; whole-conversation stats count once.
  const counted = new Set<string>();
  for (const task of tasks) {
    for (const session of task.sessions) group(session.skill_revision).sessions++;
    for (const conversation of task.conversations) {
      const key = conversation.scope === 'task' ? `${task.task_id} ${conversation.conversation_id}` : conversation.conversation_id;
      if (counted.has(key)) continue;
      counted.add(key);
      const revision = task.sessions.find((session) => session.conversation_id === conversation.conversation_id)?.skill_revision ?? 'unknown';
      const target = group(revision);
      const { stats } = conversation;
      target.conversations_with_transcript++;
      target.tool_calls += stats.tool_calls;
      target.dispatches += stats.dispatches;
      target.user_messages += stats.user_messages;
      target.notifications += stats.notifications;
      for (const kind of shellKinds) target.shell[kind] += stats.shell[kind];
      target.source_edits += stats.edits.source.calls;
      target.source_edit_bytes += stats.edits.source.bytes;
      target.source_reads += stats.reads.source;
    }
  }
  return groups;
}

// Throws when any journal cannot be read or breaks the contract, listing every
// such journal, so no metrics come from a partial set.
export function analyze(journalPaths: string[], options: AnalyzeOptions): Analysis {
  const journals = new Map<string, Journal>();
  const errors: string[] = [];
  for (const path of journalPaths) {
    try {
      const journal = readJournal(path);
      journals.set(journal.path, journal);
    } catch (error) {
      errors.push(error instanceof JournalError ? error.message : `${path}: cannot read journal: ${(error as Error).message}`);
    }
  }
  if (errors.length > 0) throw new InvalidJournalsError(errors);

  const gaps: Gap[] = [];
  const readParent = transcriptCache();
  const analyses = [...journals.values()].map((journal) => analyzeTask(journal, options, readParent, gaps));
  addConversations(analyses, readParent, gaps);

  const tasks = analyses.map(({ task }) => task);
  tasks.sort((a, b) => a.task_id.localeCompare(b.task_id));
  return { tasks, aggregates: aggregate(tasks), parent_by_skill_revision: aggregateParents(tasks), gaps };
}