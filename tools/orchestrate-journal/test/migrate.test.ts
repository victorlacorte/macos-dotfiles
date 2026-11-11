import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { test } from 'node:test';
import { parseJournal } from '../src/journal.ts';
import { migrateFile, migrateJournal } from '../src/migrate.ts';
import { attemptFinished, event, jsonl, sessionStarted, taskStarted, tempDir, write } from './fixture.ts';

const task_id = '20261104-migrate';

const { repository, utc_offset, effort, ...currentSession } = sessionStarted('conv-1');
const oldJournal = jsonl([
  event('task.started', 'parent', taskStarted, { task_id }),
  event(
    'session.started',
    'parent',
    { ...currentSession, repository_id: repository, local_utc_offset: utc_offset, skill_revision: '4844074', existing_changes: 'none outside .scratch' },
    { task_id },
  ),
  event('attempt.finished', 'hook', { ...attemptFinished('c1', '/t.jsonl'), parent_model: 'child-model' }, { task_id, attempt_id: 'work-1' }),
  event('attempt.reviewed', 'parent', { child_status: 'complete', decision: 'accepted', rationale: 'r' }, { task_id, attempt_id: 'work-1' }),
  event(
    'attempt.reviewed',
    'parent',
    { child_status: 'complete', decision: 'correction_required', reason: 'unclear_scope', rationale: 'r', corrections: 'do x' },
    { task_id, attempt_id: 'work-2' },
  ),
  event(
    'session.finished',
    'parent',
    { outcome: 'completed', criteria_met: 'all', criteria_unmet: [], open_work: [], next_steps: 'none', journal_gaps: [] },
    { task_id },
  ),
]);

test('migrates old shapes into journals the analyzer accepts without gaps', () => {
  const before = parseJournal(oldJournal, '/x', `/x/${task_id}`);
  assert.ok(before.gaps.length > 0);
  const { text, migrations } = migrateJournal(oldJournal);
  assert.deepEqual(
    migrations.map((migration) => migration.line),
    [2, 3, 4, 5, 6],
  );
  assert.deepEqual(parseJournal(text, '/x', `/x/${task_id}`).gaps, []);
  const session = JSON.parse(text.split('\n')[1] ?? '') as { data: Record<string, unknown> };
  assert.deepEqual(session.data.skill_revision, { commit: '4844074', dirty: null });
  assert.deepEqual(session.data.existing_changes, []);
  assert.equal(migrateJournal(text).migrations.length, 0);
});

test('leaves malformed and ambiguous lines untouched', () => {
  const text = ['{broken', JSON.stringify(event('session.started', 'parent', { ...sessionStarted('c'), existing_changes: 'M src/a.ts' }, { task_id })), ''].join('\n');
  const result = migrateJournal(text);
  assert.equal(result.migrations.length, 0);
  assert.equal(result.text, text);
});

test('writes in place, keeps the first original, and skips current journals', () => {
  const path = write(join(tempDir(), task_id, 'journal.jsonl'), oldJournal);
  assert.equal(migrateFile(path, false).length, 5);
  assert.equal(readFileSync(path, 'utf8'), oldJournal);
  assert.equal(migrateFile(path, true).length, 5);
  assert.equal(readFileSync(`${path}.orig`, 'utf8'), oldJournal);
  assert.notEqual(readFileSync(path, 'utf8'), oldJournal);
  assert.equal(migrateFile(path, true).length, 0);
  assert.ok(!existsSync(`${path}.migrating`));
});
