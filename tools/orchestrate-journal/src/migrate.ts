import { copyFileSync, existsSync, readFileSync, renameSync, rmSync, writeFileSync } from 'node:fs';

// Rewrites event shapes that parents and the hook wrote before the key lists
// became exact (2026-11-04) into the current schema 3 shapes. Only lossless or
// unambiguous rewrites belong here; everything else stays for the analyzer to
// report as a gap.

export interface LineMigration {
  line: number;
  changes: string[];
}

export interface JournalMigration {
  text: string;
  migrations: LineMigration[];
}

type Data = Record<string, unknown>;

const isObject = (value: unknown): value is Data => typeof value === 'object' && value !== null && !Array.isArray(value);

function rename(data: Data, from: string, to: string, changes: string[]): void {
  if (!(from in data) || to in data) return;
  data[to] = data[from];
  delete data[from];
  changes.push(`renamed ${from} to ${to}`);
}

function defaultNull(data: Data, key: string, changes: string[]): void {
  if (key in data) return;
  data[key] = null;
  changes.push(`added ${key}: null`);
}

function wrapString(data: Data, key: string, changes: string[]): void {
  if (typeof data[key] !== 'string') return;
  data[key] = [data[key]];
  changes.push(`wrapped ${key} string in a list`);
}

const rules: Readonly<Record<string, (data: Data, changes: string[]) => void>> = {
  'attempt.finished': (data, changes) => {
    if (!('parent_model' in data)) return;
    delete data.parent_model;
    changes.push("dropped parent_model (the hook recorded the child's model)");
  },
  'attempt.reviewed': (data, changes) => {
    rename(data, 'corrections', 'correction', changes);
    if (data.decision !== 'accepted') return;
    defaultNull(data, 'reason', changes);
    defaultNull(data, 'correction', changes);
  },
  'session.started': (data, changes) => {
    rename(data, 'repository_id', 'repository', changes);
    rename(data, 'local_utc_offset', 'utc_offset', changes);
    defaultNull(data, 'effort', changes);
    if (typeof data.skill_revision === 'string' && data.skill_revision !== '') {
      data.skill_revision = { commit: data.skill_revision, dirty: null };
      changes.push('converted skill_revision to {commit, dirty: null}');
    }
    if (typeof data.existing_changes === 'string' && /^none\b/i.test(data.existing_changes)) {
      changes.push(`converted existing_changes ${JSON.stringify(data.existing_changes)} to []`);
      data.existing_changes = [];
    }
  },
  'session.finished': (data, changes) => {
    rename(data, 'open_work', 'open_work_items', changes);
    for (const key of ['criteria_met', 'criteria_unmet', 'open_work_items', 'next_steps', 'journal_gaps']) {
      wrapString(data, key, changes);
    }
  },
};

export function migrateJournal(text: string): JournalMigration {
  const migrations: LineMigration[] = [];
  const lines = text.split('\n').map((raw, index) => {
    let event: unknown;
    try {
      event = JSON.parse(raw);
    } catch {
      return raw;
    }
    if (!isObject(event) || !isObject(event.data) || typeof event.type !== 'string') return raw;
    const rule = rules[event.type];
    if (rule === undefined) return raw;
    const changes: string[] = [];
    rule(event.data, changes);
    if (changes.length === 0) return raw;
    migrations.push({ line: index + 1, changes });
    return JSON.stringify(event);
  });
  return { text: lines.join('\n'), migrations };
}

// Rewrites the journal in place and keeps the first original as
// journal.jsonl.orig. Returns the migrations applied.
export function migrateFile(path: string, write: boolean): LineMigration[] {
  const original = readFileSync(path, 'utf8');
  const { text, migrations } = migrateJournal(original);
  if (!write || migrations.length === 0) return migrations;
  const backup = `${path}.orig`;
  if (!existsSync(backup)) copyFileSync(path, backup);
  const temporary = `${path}.migrating`;
  writeFileSync(temporary, text);
  if (readFileSync(path, 'utf8') !== original) {
    rmSync(temporary);
    throw new Error(`${path} changed during migration; retry when no session is writing to it`);
  }
  renameSync(temporary, path);
  return migrations;
}
