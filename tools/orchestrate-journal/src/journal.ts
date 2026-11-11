import { readFileSync, realpathSync } from 'node:fs';
import { basename, dirname } from 'node:path';
import { migrateJournal } from './migrate.ts';
import { attemptIdPattern, dataProblems, envelopeKeys, eventSpecs, schemaVersion } from './schema.ts';

export interface Gap {
  task_id: string;
  attempt_id: string | null;
  message: string;
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
  // True when the journal was refused, for example for an unknown schema version.
  skipped: boolean;
  // Current events in file order: superseded events are removed.
  events: JournalEvent[];
  // Every parsed event, superseded ones included.
  all_events: JournalEvent[];
  // session.started events in file order.
  sessions: JournalEvent[];
  gaps: Gap[];
}

const stringOrNull = (value: unknown): string | null => (typeof value === 'string' ? value : null);

const isObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

// Analyzes earlier event shapes as migrated, since migrations are lossless.
export function readJournal(path: string): Journal {
  const resolved = realpathSync(path);
  const taskDir = dirname(resolved);
  const { text, migrations } = migrateJournal(readFileSync(resolved, 'utf8'));
  const journal = parseJournal(text, resolved, taskDir);
  if (migrations.length > 0) {
    journal.gaps.unshift({
      task_id: journal.task_id,
      attempt_id: null,
      message: `${migrations.length} lines use earlier event shapes and were analyzed as migrated; \`orchestrate-journal migrate --write\` rewrites them`,
    });
  }
  return journal;
}

export function parseJournal(text: string, path: string, taskDir: string): Journal {
  const taskId = basename(taskDir);
  const gaps: Gap[] = [];
  const gap = (message: string, attemptId: string | null = null) =>
    gaps.push({ task_id: taskId, attempt_id: attemptId, message });

  const parsed: JournalEvent[] = [];
  const seenIds = new Set<string>();
  const backwards: number[] = [];
  let latest = '';
  const lines = text.split('\n');
  for (const [index, raw] of lines.entries()) {
    const line = index + 1;
    if (raw.trim() === '') continue;
    let value: unknown;
    try {
      value = JSON.parse(raw);
    } catch {
      gap(`line ${line}: malformed JSON`);
      continue;
    }
    if (!isObject(value)) {
      gap(`line ${line}: not a JSON object`);
      continue;
    }
    if (value.schema_version !== schemaVersion) {
      return {
        path,
        task_dir: taskDir,
        task_id: taskId,
        skipped: true,
        events: [],
        all_events: [],
        sessions: [],
        gaps: [
          {
            task_id: taskId,
            attempt_id: null,
            message: `line ${line}: schema_version ${JSON.stringify(value.schema_version)} is not ${schemaVersion}; journal skipped`,
          },
        ],
      };
    }

    const type = stringOrNull(value.type) ?? '';
    const attemptId = stringOrNull(value.attempt_id);
    const where = `line ${line} (${type || 'no type'})`;
    for (const key of envelopeKeys) {
      if (!(key in value)) gap(`${where}: missing envelope key ${key}`, attemptId);
    }
    for (const key of Object.keys(value)) {
      if (!envelopeKeys.includes(key)) gap(`${where}: unknown envelope key ${key}`, attemptId);
    }
    if (value.task_id !== taskId) gap(`${where}: task_id ${JSON.stringify(value.task_id)} differs from the directory name`);

    const eventId = stringOrNull(value.event_id);
    if (eventId !== null) {
      if (seenIds.has(eventId)) gap(`${where}: duplicate event_id ${eventId}`, attemptId);
      seenIds.add(eventId);
    }

    const occurredAt = stringOrNull(value.occurred_at);
    if (occurredAt !== null) {
      if (Number.isNaN(Date.parse(occurredAt))) gap(`${where}: occurred_at is not a timestamp`, attemptId);
      else if (occurredAt < latest) backwards.push(line);
      else latest = occurredAt;
    }

    const spec = eventSpecs[type];
    const data = isObject(value.data) ? value.data : {};
    if (!isObject(value.data)) gap(`${where}: data is not an object`, attemptId);
    if (spec === undefined) {
      gap(`${where}: unknown event type`, attemptId);
    } else {
      if (value.writer !== spec.writer) gap(`${where}: writer ${JSON.stringify(value.writer)} is not ${spec.writer}`, attemptId);
      if (spec.scope === 'attempt' && attemptId === null) gap(`${where}: missing attempt_id`);
      if (spec.scope === 'task' && attemptId !== null) gap(`${where}: unexpected attempt_id`, attemptId);
      for (const problem of dataProblems(spec, data)) gap(`${where}: ${problem}`, attemptId);
      if (
        type === 'attempt.reviewed' &&
        'decision' in data &&
        'reason' in data &&
        (data.decision === 'accepted') !== (data.reason === null)
      ) {
        gap(`${where}: reason must be null exactly when decision is accepted`, attemptId);
      }
    }
    if (attemptId !== null) {
      const match = attemptIdPattern.exec(attemptId);
      if (match === null) gap(`${where}: attempt_id is not <work_item_id>-<n>`, attemptId);
      else if (value.work_item_id !== match[1]) gap(`${where}: work_item_id does not match attempt_id`, attemptId);
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
  if (backwards.length > 0) {
    gap(`occurred_at goes backwards at line ${backwards.join(', ')}; file order is used`);
  }

  const byId = new Map(parsed.flatMap((event) => (event.event_id === null ? [] : [[event.event_id, event] as const])));
  const superseded = new Set<string>();
  for (const event of parsed) {
    if (event.supersedes === null) continue;
    const old = byId.get(event.supersedes);
    if (old === undefined || old.line > event.line) {
      gap(`line ${event.line}: supersedes names no earlier event: ${event.supersedes}`, event.attempt_id);
    } else if (old.type !== event.type) {
      gap(`line ${event.line}: supersedes an event of type ${old.type}`, event.attempt_id);
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
        gap(`line ${event.line} (${event.type}): recorded before any session.started`, event.attempt_id);
      }
    }
  }

  const events = parsed.filter((event) => event.event_id === null || !superseded.has(event.event_id));
  const taskStarts = events.filter((event) => event.type === 'task.started').length;
  if (taskStarts !== 1) gap(`${taskStarts} current task.started events; expected 1`);

  return { path, task_dir: taskDir, task_id: taskId, skipped: false, events, all_events: parsed, sessions, gaps };
}
