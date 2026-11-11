import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { test } from 'node:test';
import { dataProblems, eventSpecs, type FieldType } from '../src/schema.ts';
import { skillDir, tempDir } from './fixture.ts';

const sample = (type: FieldType): unknown => {
  if (typeof type !== 'string') return type[0];
  return { string: 'x', 'string?': null, strings: ['x'], 'strings?': null, 'boolean?': null, 'count?': null, 'revision?': null }[type];
};

const parentData = (type: string): Record<string, unknown> => {
  const spec = eventSpecs[type];
  assert.ok(spec);
  return Object.fromEntries(spec.inputKeys.map((key) => [key, sample(spec.fields[key] as FieldType)]));
};

const readEvents = (path: string) =>
  readFileSync(path, 'utf8')
    .trim()
    .split('\n')
    .map((line) => JSON.parse(line) as { type: string; writer: string; data: Record<string, unknown> });

test('journal-append accepts every parent event built from the analyzer schema', () => {
  const taskDir = join(tempDir(), '20261104-schema');
  mkdirSync(taskDir);
  const order = ['task.started', 'session.started', 'task.criteria_changed', 'attempt.reviewed', 'artifact.linked', 'task.completed', 'task.blocked', 'session.finished'];
  assert.deepEqual(
    [...order].sort(),
    Object.keys(eventSpecs).filter((type) => eventSpecs[type]?.writer === 'parent').sort(),
  );
  for (const type of order) {
    const args = [taskDir, type, ...(eventSpecs[type]?.scope === 'attempt' ? ['--attempt', 'work-1'] : [])];
    execFileSync(join(skillDir, 'scripts', 'journal-append'), args, {
      input: JSON.stringify(parentData(type)),
      cwd: taskDir,
      env: { ...process.env, CURSOR_CONVERSATION_ID: 'conv-1', CODEX_THREAD_ID: '' },
      stdio: ['pipe', 'pipe', 'pipe'],
    });
  }
  for (const written of readEvents(join(taskDir, 'journal.jsonl'))) {
    const spec = eventSpecs[written.type];
    assert.ok(spec);
    assert.deepEqual(dataProblems(spec, written.data), [], written.type);
  }
});

test('the Cursor hook writes exactly the attempt.finished keys', () => {
  const root = tempDir();
  const taskDir = join(root, '.scratch', '20261104-hook');
  mkdirSync(taskDir, { recursive: true });
  const input = {
    hook_event_name: 'postToolUse',
    tool_name: 'Task',
    tool_input: {
      prompt: `Orchestrate attempt\nRole: explorer\nReport: ${taskDir}/reports/map-1.md\n`,
      subagent_type: 'explore',
      model: 'child-model',
    },
    tool_output: JSON.stringify({ agentId: 'child-1', status: 'success', durationMs: 1000, isBackground: false }),
    conversation_id: 'conv-1',
    transcript_path: `${root}/agent-transcripts/conv-1/conv-1.jsonl`,
    cursor_version: '2026.10.22',
  };
  execFileSync(join(skillDir, 'hooks', 'cursor-journal.sh'), { input: JSON.stringify(input), stdio: ['pipe', 'pipe', 'pipe'] });
  const background = {
    ...input,
    tool_input: { ...input.tool_input, prompt: input.tool_input.prompt.replace('map-1', 'map-2') },
    tool_output: JSON.stringify({ agentId: 'child-2', status: 'success', isBackground: true }),
  };
  execFileSync(join(skillDir, 'hooks', 'cursor-journal.sh'), { input: JSON.stringify(background), stdio: ['pipe', 'pipe', 'pipe'] });
  const [written, launched] = readEvents(join(taskDir, 'journal.jsonl'));
  assert.ok(written && launched);
  assert.equal(written.writer, 'hook');
  const spec = eventSpecs['attempt.finished'];
  assert.ok(spec);
  assert.deepEqual(dataProblems(spec, written.data), []);
  assert.equal(launched.data.background, true);
  assert.equal(launched.data.duration_ms, null);
  assert.match(String(launched.data.started_at), /^\d{4}-\d{2}-\d{2}T[\d:]{8}Z$/);
});
