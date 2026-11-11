import assert from 'node:assert/strict';
import { join } from 'node:path';
import { test } from 'node:test';
import { attributeEntries, classifyShell, parentStats, readTranscript, type Entry } from '../src/cursor.ts';
import { tempDir, toolUse, transcript, write } from './fixture.ts';

test('classifies parent shell commands by what they run', () => {
  const cases: [string, string][] = [
    ["T=.scratch/t; cat >> $T/journal.jsonl <<'EOF'\n{}\nEOF", 'journal'],
    ['~/.agents/skills/orchestrate/scripts/journal-append .scratch/t task.started', 'journal'],
    ['yarn nx sync:check 2>&1 | tail -8', 'verification'],
    ['E=x; yarn tsc --noEmit --strict a.ts', 'verification'],
    ['npm run test -- --watch=false', 'verification'],
    ['go test ./...', 'verification'],
    ['cat .scratch/t/reports/a-1.md; git status --short', 'task_artifacts'],
    ['git diff --stat; gh pr view 1', 'git_gh'],
    ['cd repo && git log --oneline -5', 'git_gh'],
    ['head -5 apps/member/src/pulse-assets.test.ts', 'exploration'],
    ['rg -n "lint" src | head', 'exploration'],
    ["cat > /tmp/pr.md <<'EOF'\ntest the thing\nlint it\nEOF", 'exploration'],
    ['open https://example.com', 'other'],
  ];
  for (const [command, kind] of cases) assert.equal(classifyShell(command), kind, command);
});

test('parent stats unwrap dynamic Task calls and split edits by scope', () => {
  const dir = tempDir();
  const taskDir = join(dir, 'repo', '.scratch', '20261104-t');
  const path = write(
    join(dir, 'parent.jsonl'),
    transcript([
      { role: 'user', content: [{ type: 'text', text: '<manually_attached_skills>orchestrate</manually_attached_skills>' }] },
      { role: 'user', content: [{ type: 'text', text: '<timestamp>t</timestamp>\n<user_query>\ngo\n</user_query>' }] },
      {
        role: 'user',
        content: [{ type: 'text', text: '<user_query>Perform any necessary follow-up actions in response to the subagent completion above.</user_query>' }],
      },
      {
        role: 'assistant',
        content: [
          { type: 'text', text: 'dispatching' },
          toolUse('CallDynamicTool', { namespace: 'cursor', toolName: 'Task', arguments: { prompt: 'p' } }),
          toolUse('CallDynamicTool', { namespace: 'linear', toolName: 'get_issue', arguments: {} }),
          toolUse('Write', { path: `${taskDir}/briefs/a-1.md`, contents: 'brief' }),
          toolUse('StrReplace', { path: `${dir}/repo/src/a.ts`, old_string: 'a', new_string: 'abc' }),
          toolUse('Read', { path: `${dir}/repo/.scratch/other/plan.md` }),
          toolUse('Shell', { command: 'yarn tsc --noEmit' }),
        ],
      },
    ]),
  );
  const parsed = readTranscript(path);
  assert.equal(parsed.turn_status, 'success');
  const stats = parentStats(parsed.entries, taskDir);
  assert.equal(stats.dispatches, 1);
  assert.equal(stats.tools['linear.get_issue'], 1);
  assert.deepEqual(stats.edits.task, { calls: 1, bytes: 5 });
  assert.deepEqual(stats.edits.source, { calls: 1, bytes: 3 });
  assert.equal(stats.reads.scratch, 1);
  assert.equal(stats.shell.verification, 1);
  assert.equal(stats.user_messages, 1);
  assert.equal(stats.notifications, 1);
});

test('attributes a shared conversation to the tasks its calls name', () => {
  const tool = (name: string, input: Record<string, unknown>): Entry => ({ kind: 'tool', use: { name, input } });
  const read = tool('Read', { path: '/repo/src/a.ts' });
  const entries: Entry[] = [
    { kind: 'query' },
    read,
    tool('Shell', { command: 'T=$PWD/.scratch/20261106-a\n$J "$T" session.started <<EOF\n{}\nEOF' }),
    tool('Task', { prompt: 'Orchestrate attempt\nReport: /r/.scratch/20261106-a/reports/map-1.md\n' }),
    { kind: 'query' },
    tool('Shell', { command: 'T=/r/.scratch/20261106-ab\n$J "$T" task.started <<EOF\n{"derived_from": "/r/.scratch/20261106-a"}\nEOF' }),
    tool('Write', { path: '/r/.scratch/rent-reporting/home.md', contents: 'docs' }),
    { kind: 'notification' },
    tool('Read', { path: '/r/.scratch/20261106-a/reports/map-1.md' }),
    tool('Shell', { command: 'git diff --stat' }),
  ];
  const parts = attributeEntries(entries, ['20261106-a', '20261106-ab']);
  assert.deepEqual(
    [...(parts ?? [])].map(([taskId, part]) => [taskId, part.map((entry) => entries.indexOf(entry))]),
    [
      ['20261106-a', [0, 1, 2, 3, 7, 8, 9]],
      ['20261106-ab', [4, 5, 6]],
    ],
  );
  assert.equal(attributeEntries(entries, ['20261106-a', '20261106-c']), null);
});
