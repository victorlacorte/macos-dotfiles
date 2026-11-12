import assert from 'node:assert/strict';
import { test } from 'node:test';
import { JournalError, parseJournal } from '../src/journal.ts';
import { event, jsonl, review, sessionFinished, sessionStarted, taskStarted } from './fixture.ts';

const task_id = '20261104-journal';
const parse = (text: string) => parseJournal(text, `/x/${task_id}/journal.jsonl`, `/x/${task_id}`);

function problems(text: string): readonly string[] {
  try {
    parse(text);
  } catch (error) {
    if (error instanceof JournalError) return error.problems;
    throw error;
  }
  assert.fail('expected a JournalError');
}

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
  const reviews = journal.events.filter((item) => item.type === 'attempt.reviewed');
  assert.equal(reviews.length, 1);
  assert.equal(reviews[0]?.data.decision, 'correction_required');
  assert.equal(reviews[0]?.session, 0);
  assert.equal(journal.sessions.length, 2);
});

test('rejects malformed lines and unknown keys, listing every problem, but accepts backwards timestamps', () => {
  const found = problems(
    [
      JSON.stringify(event('task.started', 'parent', { ...taskStarted, context: 'extra' }, { task_id })),
      '{not json',
      JSON.stringify(event('session.started', 'parent', sessionStarted('conv-1'), { task_id, occurred_at: '2026-11-04T13:00:00Z' })),
      JSON.stringify(event('attempt.reviewed', 'parent', { ...review('accepted'), reason: 'unknown' }, { task_id, attempt_id: 'work-1', occurred_at: '2026-11-04T12:00:00Z' })),
      '',
    ].join('\n'),
  );
  assert.ok(found.includes('line 1 (task.started): unknown key: context'));
  assert.ok(found.includes('line 2: malformed JSON'));
  assert.ok(found.includes('line 4 (attempt.reviewed): reason must be null exactly when decision is accepted'));
  assert.ok(!found.some((problem) => problem.includes('occurred_at')));
});

test('rejects a journal with another schema version and an unknown dirty state', () => {
  assert.deepEqual(problems(jsonl([{ ...event('task.started', 'parent', taskStarted, { task_id }), schema_version: 3 }])), [
    'line 1: schema_version 3 is not 4',
  ]);
  const found = problems(
    jsonl([
      event('task.started', 'parent', taskStarted, { task_id }),
      event('session.started', 'parent', sessionStarted('conv-1', { commit: 'abc', dirty: null }), { task_id }),
    ]),
  );
  assert.deepEqual(found, ['line 2 (session.started): skill_revision must be null or {commit, dirty}']);
});

test('rejects events outside a session and supersedes that name nothing', () => {
  const found = problems(
    jsonl([
      event('task.started', 'parent', taskStarted, { task_id }),
      event('artifact.linked', 'parent', { kind: 'pr', relationship: 'r', locator: 'l' }, { task_id }),
      event('session.started', 'parent', sessionStarted('conv-1'), { task_id }),
      event('artifact.linked', 'parent', { kind: 'pr', relationship: 'r', locator: 'l' }, { task_id, supersedes: 'missing' }),
    ]),
  );
  assert.ok(found.includes('line 2 (artifact.linked): recorded before any session.started'));
  assert.ok(found.includes('line 4: supersedes names no earlier event: missing'));
});

test('rejects a second current review of an attempt and a second session.finished in a session', () => {
  const found = problems(
    jsonl([
      event('task.started', 'parent', taskStarted, { task_id }),
      event('session.started', 'parent', sessionStarted('conv-1'), { task_id }),
      event('attempt.reviewed', 'parent', review('accepted'), { task_id, attempt_id: 'work-1' }),
      event('attempt.reviewed', 'parent', review('accepted'), { task_id, attempt_id: 'work-1' }),
      event('session.finished', 'parent', sessionFinished, { task_id }),
      event('session.finished', 'parent', sessionFinished, { task_id }),
    ]),
  );
  assert.deepEqual(found, [
    '2 current attempt.reviewed events for attempt work-1; expected 1',
    '2 current session.finished events for session 1; expected 1',
  ]);
});
