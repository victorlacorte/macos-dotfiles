import { readFileSync, realpathSync } from 'node:fs';
import { basename, dirname } from 'node:path';
import { attemptIdPattern, dataProblems, envelopeKeys, eventSpecs, schemaVersion } from './schema.ts';

export class JournalError extends Error {
  readonly problems: readonly string[];

  constructor(path: string, problems: string[]) {
    super(`${path}:\n${problems.map((problem) => `  ${problem}`).join('\n')}`);
    this.problems = problems;
  }
}

export interface JournalEvent {
  line: number;
  bytes: number;
  event_id: string | null;
  occurred_at: string | null;
  type: string;
  writer: string | null;
  attempt_id: string | null;
  supersedes: string | null;
  data: Record<string, unknown>;
  // Index into Journal.sessions; null before the first session.started.
  session: number | null;
}

export interface Journal {
  path: string;
  task_dir: string;
  task_id: string;
  // Current events in file order: superseded events are removed.
  events: JournalEvent[];
  // Every parsed event, superseded ones included.
  all_events: JournalEvent[];
  // session.started events in file order.
  sessions: JournalEvent[];
}

const stringOrNull = (value: unknown): string | null => (typeof value === 'string' ? value : null);

const isObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

export function readJournal(path: string): Journal {
  const resolved = realpathSync(path);
  return parseJournal(readFileSync(resolved, 'utf8'), resolved, dirname(resolved));
}

// Throws a JournalError listing every problem when the journal breaks the contract.
export function parseJournal(text: string, path: string, taskDir: string): Journal {
  const taskId = basename(taskDir);
  const problems: string[] = [];
  const problem = (message: string) => problems.push(message);

  const parsed: JournalEvent[] = [];
  const seenIds = new Set<string>();
  const lines = text.split('\n');
  for (const [index, raw] of lines.entries()) {
    const line = index + 1;
    if (raw.trim() === '') continue;
    let value: unknown;
    try {
      value = JSON.parse(raw);
    } catch {
      problem(`line ${line}: malformed JSON`);
      continue;
    }
    if (!isObject(value)) {
      problem(`line ${line}: not a JSON object`);
      continue;
    }
    if (value.schema_version !== schemaVersion) {
      throw new JournalError(path, [`line ${line}: schema_version ${JSON.stringify(value.schema_version)} is not ${schemaVersion}`]);
    }

    const type = stringOrNull(value.type) ?? '';
    const attemptId = stringOrNull(value.attempt_id);
    const where = `line ${line} (${type || 'no type'})`;
    for (const key of envelopeKeys) {
      if (!(key in value)) problem(`${where}: missing envelope key ${key}`);
    }
    for (const key of Object.keys(value)) {
      if (!envelopeKeys.includes(key)) problem(`${where}: unknown envelope key ${key}`);
    }
    if (value.task_id !== taskId) problem(`${where}: task_id ${JSON.stringify(value.task_id)} differs from the directory name`);

    const eventId = stringOrNull(value.event_id);
    if (eventId !== null) {
      if (seenIds.has(eventId)) problem(`${where}: duplicate event_id ${eventId}`);
      seenIds.add(eventId);
    }

    // Do not require ascending occurred_at: file order is event order, and the parent
    // and hook stamp before appending, so concurrent writes can land out of order.
    const occurredAt = stringOrNull(value.occurred_at);
    if (occurredAt !== null && Number.isNaN(Date.parse(occurredAt))) problem(`${where}: occurred_at is not a timestamp`);

    const spec = eventSpecs[type];
    const data = isObject(value.data) ? value.data : {};
    if (!isObject(value.data)) problem(`${where}: data is not an object`);
    if (spec === undefined) {
      problem(`${where}: unknown event type`);
    } else {
      if (value.writer !== spec.writer) problem(`${where}: writer ${JSON.stringify(value.writer)} is not ${spec.writer}`);
      if (spec.scope === 'attempt' && attemptId === null) problem(`${where}: missing attempt_id`);
      if (spec.scope === 'task' && attemptId !== null) problem(`${where}: unexpected attempt_id`);
      for (const dataProblem of dataProblems(spec, data)) problem(`${where}: ${dataProblem}`);
      if (
        type === 'attempt.reviewed' &&
        'decision' in data &&
        'reason' in data &&
        (data.decision === 'accepted') !== (data.reason === null)
      ) {
        problem(`${where}: reason must be null exactly when decision is accepted`);
      }
    }
    if (attemptId !== null) {
      const match = attemptIdPattern.exec(attemptId);
      if (match === null) problem(`${where}: attempt_id is not <work_item_id>-<n>`);
      else if (value.work_item_id !== match[1]) problem(`${where}: work_item_id does not match attempt_id`);
    }

    parsed.push({
      line,
      bytes: Buffer.byteLength(raw, 'utf8') + 1,
      event_id: eventId,
      occurred_at: occurredAt,
      type,
      writer: stringOrNull(value.writer),
      attempt_id: attemptId,
      supersedes: stringOrNull(value.supersedes),
      data,
      session: null,
    });
  }

  const byId = new Map(parsed.flatMap((event) => (event.event_id === null ? [] : [[event.event_id, event] as const])));
  const superseded = new Set<string>();
  for (const event of parsed) {
    if (event.supersedes === null) continue;
    const old = byId.get(event.supersedes);
    if (old === undefined || old.line > event.line) {
      problem(`line ${event.line}: supersedes names no earlier event: ${event.supersedes}`);
    } else if (old.type !== event.type) {
      problem(`line ${event.line}: supersedes an event of type ${old.type}`);
    } else {
      superseded.add(event.supersedes);
    }
  }

  // A correction belongs to the session of the event it replaces.
  const sessions: JournalEvent[] = [];
  for (const event of parsed) {
    const replaced = event.supersedes === null ? undefined : byId.get(event.supersedes);
    if (replaced !== undefined && replaced.line < event.line) {
      event.session = replaced.session;
      if (event.type === 'session.started') {
        const index = replaced.session;
        if (index !== null) sessions[index] = event;
      }
    } else if (event.type === 'session.started') {
      sessions.push(event);
      event.session = sessions.length - 1;
    } else {
      event.session = sessions.length === 0 ? null : sessions.length - 1;
      if (event.session === null && event.type !== 'task.started') {
        problem(`line ${event.line} (${event.type}): recorded before any session.started`);
      }
    }
  }

  const events = parsed.filter((event) => event.event_id === null || !superseded.has(event.event_id));
  const taskStarts = events.filter((event) => event.type === 'task.started').length;
  if (taskStarts !== 1) problem(`${taskStarts} current task.started events; expected 1`);
  const repeated = (type: string, key: (event: JournalEvent) => string | number | null, describe: string) => {
    const counts = new Map<string | number | null, number>();
    for (const event of events) if (event.type === type) counts.set(key(event), (counts.get(key(event)) ?? 0) + 1);
    for (const [value, count] of counts) if (count > 1) problem(`${count} current ${type} events for ${describe} ${value}; expected 1`);
  };
  repeated('attempt.reviewed', (event) => event.attempt_id, 'attempt');
  repeated('session.finished', (event) => (event.session === null ? null : event.session + 1), 'session');

  if (problems.length > 0) throw new JournalError(path, problems);
  return { path, task_dir: taskDir, task_id: taskId, events, all_events: parsed, sessions };
}
