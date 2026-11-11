import assert from 'node:assert/strict';
import { test } from 'node:test';
import { parseJournal } from '../src/journal.ts';
import { event, jsonl, review, sessionFinished, sessionStarted, taskStarted } from './fixture.ts';

const task_id = '20261104-journal';
const parse = (text: string) => parseJournal(text, `/x/${task_id}/journal.jsonl`, `/x/${task_id}`);

test('applies supersedes and assigns corrections to the replaced event session', () => {
  const journal = parse(
    jsonl([
      event('task.started', 'parent', taskStarted, { task_id }),
      event('session.started', 'parent', sessionStarted('conv-1'), { task_id }),
      event('attempt.reviewed', 'parent', review('accepted'), { task_id, attempt_id: 'work-1', event_id: 'r1' }),
      event('session.finished', 'parent', sessionFinished, { task_id }),
      event('session.started', 'parent', sessionStarted('conv-2'), { task_id }),
      event('attempt.reviewed', 'parent', review('correction_required', 'missing_context'), {
        task_id,
        attempt_id: 'work-1',
        supersedes: 'r1',
      }),
    ]),
  );
  assert.deepEqual(journal.gaps, []);
  const reviews = journal.events.filter((item) => item.type === 'attempt.reviewed');
  assert.equal(reviews.length, 1);
  assert.equal(reviews[0]?.data.decision, 'correction_required');
  assert.equal(reviews[0]?.session, 0);
  assert.equal(journal.sessions.length, 2);
});

test('reports malformed lines, unknown keys, and backwards timestamps without dropping valid events', () => {
  const journal = parse(
    [
      JSON.stringify(event('task.started', 'parent', { ...taskStarted, context: 'extra' }, { task_id })),
      '{not json',
      JSON.stringify(event('session.started', 'parent', sessionStarted('conv-1'), { task_id, occurred_at: '2026-11-04T13:00:00Z' })),
      JSON.stringify(event('attempt.reviewed', 'parent', { ...review('accepted'), reason: 'unknown' }, { task_id, attempt_id: 'work-1', occurred_at: '2026-11-04T12:00:00Z' })),
      '',
    ].join('\n'),
  );
  const messages = journal.gaps.map((gap) => gap.message);
  assert.ok(messages.includes('line 1 (task.started): unknown key: context'));
  assert.ok(messages.includes('line 2: malformed JSON'));
  assert.ok(messages.includes('line 4 (attempt.reviewed): reason must be null exactly when decision is accepted'));
  assert.ok(messages.includes('occurred_at goes backwards at line 4; file order is used'));
  assert.equal(journal.events.length, 3);
});

test('refuses a journal with an unknown schema version', () => {
  const journal = parse(jsonl([{ ...event('task.started', 'parent', taskStarted, { task_id }), schema_version: 4 }]));
  assert.equal(journal.skipped, true);
  assert.equal(journal.events.length, 0);
  assert.match(journal.gaps[0]?.message ?? '', /schema_version 4 is not 3; journal skipped/);
});

test('flags events outside a session and supersedes that name nothing', () => {
  const journal = parse(
    jsonl([
      event('task.started', 'parent', taskStarted, { task_id }),
      event('artifact.linked', 'parent', { kind: 'pr', relationship: 'r', locator: 'l' }, { task_id }),
      event('session.started', 'parent', sessionStarted('conv-1'), { task_id }),
      event('artifact.linked', 'parent', { kind: 'pr', relationship: 'r', locator: 'l' }, { task_id, supersedes: 'missing' }),
    ]),
  );
  const messages = journal.gaps.map((gap) => gap.message);
  assert.ok(messages.includes('line 2 (artifact.linked): recorded before any session.started'));
  assert.ok(messages.includes('line 4: supersedes names no earlier event: missing'));
});
