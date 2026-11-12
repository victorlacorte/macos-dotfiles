import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { utimesSync } from 'node:fs';
import { join } from 'node:path';
import { test } from 'node:test';
import { analyze, InvalidJournalsError, type Analysis } from '../src/analyze.ts';
import { renderMarkdown } from '../src/render.ts';
import {
  attemptFinished,
  event,
  jsonl,
  review,
  sessionFinished,
  sessionStarted,
  taskStarted,
  tempDir,
  toolUse,
  transcript,
  write,
} from './fixture.ts';

const task_id = '20261104-analyze';

function buildFixture() {
  const root = tempDir();
  const taskDir = join(root, 'repo', '.scratch', task_id);
  const cursorDir = join(root, 'cursor');
  const transcripts = join(cursorDir, 'projects', 'repo', 'agent-transcripts', 'conv-1');
  write(
    join(transcripts, 'conv-1.jsonl'),
    transcript([
      { role: 'user', content: [{ type: 'text', text: 'orchestrate' }] },
      {
        role: 'assistant',
        content: [
          toolUse('Task', { prompt: 'p' }),
          toolUse('Task', { prompt: 'p' }),
          toolUse('Task', { prompt: 'p' }),
          toolUse('Shell', { command: `cat ${taskDir}/reports/implement-1.md` }),
          toolUse('Shell', { command: 'yarn jest src' }),
          toolUse('StrReplace', { path: join(root, 'repo', 'src', 'a.ts'), old_string: 'a', new_string: 'b' }),
        ],
      },
    ]),
  );
  const child = (id: string, calls: number, followUp = false) =>
    write(
      join(transcripts, 'subagents', `${id}.jsonl`),
      transcript([
        { role: 'user', content: [{ type: 'text', text: '<user_query>brief</user_query>' }] },
        { role: 'user', content: [{ type: 'text', text: '<available_subagent_types>injected</available_subagent_types>' }] },
        { role: 'assistant', content: Array.from({ length: calls }, () => toolUse('Read', { path: '/x' })) },
        ...(followUp ? [{ role: 'user' as const, content: [{ type: 'text', text: '<user_query>more</user_query>' }] }] : []),
      ]),
    );
  for (const [attempt, role] of [
    ['implement-1', 'implementer'],
    ['implement-2', 'implementer'],
    ['verify-1', 'explorer'],
    ['verify-2', 'explorer'],
  ] as const) {
    write(join(taskDir, 'briefs', `${attempt}.md`), `Orchestrate attempt\nRole: ${role}\nReport: ${taskDir}/reports/${attempt}.md\n`);
    if (attempt !== 'verify-2') write(join(taskDir, 'reports', `${attempt}.md`), 'report body\n');
  }
  const journalPath = write(
    join(taskDir, 'journal.jsonl'),
    jsonl([
      event('task.started', 'parent', taskStarted, { task_id }),
      event('session.started', 'parent', sessionStarted('conv-1'), { task_id }),
      event('attempt.finished', 'hook', attemptFinished('c1', child('c1', 10)), { task_id, attempt_id: 'implement-1' }),
      event('attempt.reviewed', 'parent', review('correction_required', 'implementation_defect'), { task_id, attempt_id: 'implement-1' }),
      event('attempt.finished', 'hook', attemptFinished('c2', child('c2', 4, true), { duration_ms: 30000 }), {
        task_id,
        attempt_id: 'implement-2',
      }),
      event('attempt.reviewed', 'parent', review('accepted'), { task_id, attempt_id: 'implement-2' }),
      event(
        'attempt.finished',
        'hook',
        attemptFinished('c3', join(transcripts, 'subagents', 'gone.jsonl'), { role: 'explorer', background: true, duration_ms: null }),
        { task_id, attempt_id: 'verify-1' },
      ),
      event('attempt.reviewed', 'parent', review('accepted'), { task_id, attempt_id: 'verify-1' }),
      event('task.completed', 'parent', { outcome: 'done', artifacts: [] }, { task_id }),
      event('session.finished', 'parent', sessionFinished, { task_id }),
    ]),
  );
  return { journalPath, cursorDir, taskDir };
}

const messages = (analysis: Analysis) => analysis.gaps.map((gap) => `${gap.attempt_id ?? '-'}: ${gap.message}`);

test('builds attempt, work item, and parent records with gaps', () => {
  const { journalPath, cursorDir } = buildFixture();
  const analysis = analyze([journalPath, journalPath], { cursor_dir: cursorDir });
  assert.equal(analysis.tasks.length, 1);
  const [task] = analysis.tasks;
  assert.ok(task);
  assert.equal(task.state, 'completed');
  assert.deepEqual(
    task.attempts.map((attempt) => [attempt.attempt_id, attempt.role, attempt.decision, attempt.child?.tool_calls ?? null]),
    [
      ['implement-1', 'implementer', 'correction_required', 10],
      ['implement-2', 'implementer', 'accepted', 4],
      ['verify-1', 'explorer', 'accepted', null],
      ['verify-2', 'explorer', null, null],
    ],
  );
  assert.deepEqual(task.work_items, [
    { work_item_id: 'implement', roles: ['implementer'], attempts: 2, corrections: 1, first_attempt_accepted: false, latest_decision: 'accepted' },
    { work_item_id: 'verify', roles: ['explorer'], attempts: 2, corrections: 0, first_attempt_accepted: true, latest_decision: null },
  ]);
  const parent = task.conversations[0]?.stats;
  assert.equal(parent?.dispatches, 3);
  assert.equal(parent?.shell.task_artifacts, 1);
  assert.equal(parent?.shell.verification, 1);
  assert.equal(parent?.edits.source.calls, 1);

  const gaps = messages(analysis);
  assert.ok(gaps.some((gap) => gap.startsWith('verify-1: child transcript missing')));
  assert.ok(gaps.includes('verify-2: report missing'));
  assert.ok(gaps.includes('verify-2: no attempt.finished; the hook did not record it'));
  assert.ok(gaps.includes('verify-2: not reviewed'));
  assert.equal(gaps.length, 4);

  const implementer = analysis.aggregates.role.implementer;
  assert.equal(implementer?.attempts, 2);
  assert.equal(implementer?.first_attempts_accepted, 0);
  assert.equal(implementer?.first_attempts_reviewed, 1);
  assert.equal(implementer?.duration_ms_median, 45000);
  assert.equal(implementer?.child_follow_ups, 1);
  assert.deepEqual(implementer?.reasons, { implementation_defect: 1 });
  assert.equal(analysis.aggregates.skill_revision.abcdef012345?.attempts, 4);
  assert.equal(analysis.parent_by_skill_revision.abcdef012345?.source_edits, 1);
});

test('renders Markdown with the task table and gaps', () => {
  const { journalPath, cursorDir } = buildFixture();
  const markdown = renderMarkdown(analyze([journalPath], { cursor_dir: cursorDir }));
  assert.match(markdown, /^Tasks 1 \(completed 1\); sessions 1; attempts 4 over 2 work items; gaps 4\.$/m);
  assert.match(markdown, /^\| implementer \| 2 \| 0\/1 \| 1 \| implementation_defect 1 \| 45 \| 90 \(2\) \|/m);
  assert.match(markdown, /^\| verify-1 \| explorer \| child-model \| bg \| - \|/m);
  assert.match(markdown, /^- 20261104-analyze verify-2: not reviewed$/m);
});

test('splits a shared parent conversation by task and estimates background durations', () => {
  const root = tempDir();
  const cursorDir = join(root, 'cursor');
  const transcripts = join(cursorDir, 'projects', 'repo', 'agent-transcripts', 'conv-1');
  const taskDir = (id: string) => join(root, 'repo', '.scratch', id);
  const start = (id: string) => toolUse('Shell', { command: `T=${taskDir(id)}\njournal-append "$T" session.started <<'EOF'\n{}\nEOF` });
  write(
    join(transcripts, 'conv-1.jsonl'),
    transcript([
      { role: 'user', content: [{ type: 'text', text: '<user_query>first</user_query>' }] },
      { role: 'assistant', content: [start('20261106-a'), toolUse('Task', { prompt: 'p' }), toolUse('Task', { prompt: 'p' })] },
      { role: 'user', content: [{ type: 'text', text: '<user_query>second</user_query>' }] },
      { role: 'assistant', content: [start('20261106-b'), toolUse('Task', { prompt: 'p' })] },
    ]),
  );
  const childPath = write(join(transcripts, 'subagents', 'c1.jsonl'), transcript([]));
  utimesSync(childPath, new Date('2026-11-06T12:05:00Z'), new Date('2026-11-06T12:05:00Z'));
  const journals = ['20261106-a', '20261106-b'].map((id) =>
    write(
      join(taskDir(id), 'journal.jsonl'),
      jsonl([
        event('task.started', 'parent', taskStarted, { task_id: id }),
        event('session.started', 'parent', { ...sessionStarted('conv-1'), model: 'Claude-Opus-5.5' }, { task_id: id }),
        ...(id === '20261106-a'
          ? [
              event('attempt.finished', 'hook', attemptFinished('c1', childPath, { background: true, duration_ms: null, started_at: '2026-11-06T12:00:00Z' }), {
                task_id: id,
                attempt_id: 'map-1',
              }),
            ]
          : []),
      ]),
    ),
  );
  const analysis = analyze(journals, { cursor_dir: cursorDir });
  const [a, b] = analysis.tasks;
  assert.deepEqual(
    [a, b].map((task) => [task?.conversations[0]?.scope, task?.conversations[0]?.stats.dispatches, task?.conversations[0]?.stats.user_messages]),
    [
      ['task', 2, 1],
      ['task', 1, 1],
    ],
  );
  assert.ok(!messages(analysis).some((gap) => gap.includes('also serves')));
  assert.equal(analysis.parent_by_skill_revision.abcdef012345?.dispatches, 3);
  assert.deepEqual(Object.keys(analysis.aggregates.parent_model), ['claude-opus-5-5']);
  const [attempt] = a?.attempts ?? [];
  assert.equal(attempt?.duration_ms, 300000);
  assert.equal(attempt?.duration_estimated, true);
  assert.match(renderMarkdown(analysis), /^\| map-1 \| implementer \| child-model \| ~300 \|/m);
});

test('recovers dispatch fields from the parent transcript when the hook missed an attempt', () => {
  const root = tempDir();
  const taskDir = join(root, 'repo', '.scratch', task_id);
  const cursorDir = join(root, 'cursor');
  const header = (attempt: string) =>
    `Orchestrate attempt\nRole: explorer\nReport: ${taskDir}/reports/${attempt}.md\nEvidence: ${taskDir}/evidence/${attempt}/\nWorking directory: /work`;
  const pointer = (attempt: string) => `Read your full brief first and follow it exactly: ${taskDir}/briefs/${attempt}.md`;
  write(
    join(cursorDir, 'projects', 'repo', 'agent-transcripts', 'conv-1', 'conv-1.jsonl'),
    transcript([
      { role: 'user', content: [{ type: 'text', text: '<user_query>orchestrate</user_query>' }] },
      {
        role: 'assistant',
        content: [
          toolUse('Task', { prompt: pointer('trace-1'), model: 'early-model', subagent_type: 'explore' }),
          toolUse('Task', { prompt: pointer('trace-1'), model: 'child-model', subagent_type: 'explore', run_in_background: true }),
          toolUse('Task', { prompt: `${header('trace-2')}\nPrevious brief: ${taskDir}/briefs/trace-1.md`, model: 'other-model', subagent_type: 'generalPurpose' }),
          toolUse('CallDynamicTool', {
            namespace: 'cursor',
            toolName: 'Task',
            arguments: { prompt: `${header('verify-1')}\n${pointer('verify-1')}`, model: 'verify-model', subagent_type: 'explore' },
          }),
        ],
      },
    ]),
  );
  for (const attempt of ['trace-1', 'trace-2', 'verify-1', 'lost-1']) {
    write(join(taskDir, 'briefs', `${attempt}.md`), `${header(attempt)}\n`);
    write(join(taskDir, 'reports', `${attempt}.md`), 'report body\n');
  }
  const journalPath = write(
    join(taskDir, 'journal.jsonl'),
    jsonl([
      event('task.started', 'parent', taskStarted, { task_id }),
      event('session.started', 'parent', sessionStarted('conv-1'), { task_id }),
      ...['trace-1', 'trace-2', 'verify-1', 'lost-1'].map((attempt_id) => event('attempt.reviewed', 'parent', review('accepted'), { task_id, attempt_id })),
    ]),
  );
  const analysis = analyze([journalPath], { cursor_dir: cursorDir });
  assert.deepEqual(
    analysis.tasks[0]?.attempts.map((attempt) => [
      attempt.attempt_id,
      attempt.child_type,
      attempt.requested_model,
      attempt.background,
      attempt.dispatch_source,
      attempt.native_child_id,
      attempt.duration_ms,
    ]),
    [
      ['trace-1', 'explore', 'child-model', true, 'parent_transcript', null, null],
      ['trace-2', 'generalPurpose', 'other-model', false, 'parent_transcript', null, null],
      ['verify-1', 'explore', 'verify-model', false, 'parent_transcript', null, null],
      ['lost-1', null, null, null, null, null, null],
    ],
  );
  assert.deepEqual(messages(analysis).sort(), [
    'lost-1: no attempt.finished; the hook did not record it',
    'trace-1: no attempt.finished; dispatch fields recovered from the parent transcript',
    'trace-2: no attempt.finished; dispatch fields recovered from the parent transcript',
    'verify-1: no attempt.finished; dispatch fields recovered from the parent transcript',
  ]);
});

function writeInvalidJournal(root: string): string {
  const id = '20261111-invalid';
  return write(join(root, id, 'journal.jsonl'), jsonl([event('task.started', 'parent', { ...taskStarted, context: 'extra' }, { task_id: id })]));
}

test('refuses to analyze when any journal is invalid, listing every invalid journal', () => {
  const { journalPath, cursorDir } = buildFixture();
  const root = tempDir();
  const invalid = writeInvalidJournal(root);
  const missing = join(root, 'missing', 'journal.jsonl');
  assert.throws(() => analyze([journalPath, invalid, missing], { cursor_dir: cursorDir }), (error: unknown) => {
    assert.ok(error instanceof InvalidJournalsError);
    assert.match(error.message, /^invalid journals\n/);
    assert.match(error.message, /20261111-invalid\/journal\.jsonl:\n {2}line 1 \(task\.started\): unknown key: context/);
    assert.match(error.message, /missing\/journal\.jsonl: cannot read journal: ENOENT/);
    return true;
  });
});

test('CLI prints JSON, fails on invalid journals, and rejects missing arguments', () => {
  const { journalPath, cursorDir } = buildFixture();
  const cli = join(import.meta.dirname, '..', 'src', 'cli.ts');
  const output = execFileSync(cli, ['analyze', '--json', '--cursor-dir', cursorDir, journalPath], { encoding: 'utf8' });
  assert.equal((JSON.parse(output) as Analysis).tasks[0]?.attempts.length, 4);
  assert.throws(() => execFileSync(cli, ['analyze', '--cursor-dir', cursorDir, writeInvalidJournal(tempDir())], { stdio: 'pipe' }), {
    status: 1,
    stdout: Buffer.alloc(0),
  });
  assert.throws(() => execFileSync(cli, ['analyze'], { stdio: 'pipe' }), { status: 2 });
  assert.throws(() => execFileSync(cli, ['migrate', journalPath], { stdio: 'pipe' }), { status: 2 });
});
