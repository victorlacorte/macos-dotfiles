import type { Analysis, AttemptRecord, Dimension, GroupStats, TaskRecord } from './analyze.ts';
import { dimensions } from './analyze.ts';
import { shellKinds } from './cursor.ts';

const dimensionTitles: Record<Dimension, string> = {
  role: 'role',
  requested_model: 'requested child model',
  skill_revision: 'skill revision',
  parent_model: 'parent model',
};

const kb = (bytes: number | null): string =>
  bytes === null ? '-' : bytes > 0 && bytes < 103 ? '<0.1' : (bytes / 1024).toFixed(1);
const seconds = (ms: number | null): string => (ms === null ? '-' : String(Math.round(ms / 1000)));
const cell = (value: string | number | null): string =>
  value === null || value === '' ? '-' : String(value).replaceAll('|', '\\|').replaceAll('\n', ' ');
const counts = (record: Record<string, number>): string =>
  Object.entries(record)
    .filter(([, count]) => count > 0)
    .sort(([a, x], [b, y]) => y - x || a.localeCompare(b))
    .map(([key, count]) => `${key} ${count}`)
    .join(', ') || '-';

function table(header: string[], rows: (string | number | null)[][]): string[] {
  return [`| ${header.join(' | ')} |`, `| ${header.map(() => '---').join(' | ')} |`, ...rows.map((row) => `| ${row.map(cell).join(' | ')} |`)];
}

function groupRow(key: string, stats: GroupStats): (string | number | null)[] {
  return [
    key,
    stats.attempts,
    `${stats.first_attempts_accepted}/${stats.first_attempts_reviewed}`,
    stats.corrections,
    counts(stats.reasons),
    seconds(stats.duration_ms_median),
    `${seconds(stats.duration_ms_total)} (${stats.durations_known}${stats.durations_estimated > 0 ? `, ~${stats.durations_estimated}` : ''})`,
    stats.child_tool_calls_median,
    `${stats.child_tool_calls_total} (${stats.child_tool_calls_known})`,
    stats.child_follow_ups,
    kb(stats.brief_bytes),
    kb(stats.report_bytes),
    counts(stats.non_success),
  ];
}

const groupHeader = [
  'group',
  'attempts',
  'first accepted',
  'corrections',
  'reasons',
  'median s',
  'total s (known)',
  'median calls',
  'total calls (known)',
  'follow-ups',
  'brief KB',
  'report KB',
  'non-success',
];

function attemptRow(attempt: AttemptRecord): (string | number | null)[] {
  const status = [
    attempt.harness_status !== null && attempt.harness_status !== 'success' ? `harness ${attempt.harness_status}` : null,
    attempt.child_status !== null && attempt.child_status !== 'complete' ? `child ${attempt.child_status}` : null,
  ].filter((value) => value !== null);
  return [
    attempt.attempt_id,
    attempt.role,
    attempt.requested_model,
    attempt.duration_estimated ? `~${seconds(attempt.duration_ms)}` : attempt.background === true ? 'bg' : seconds(attempt.duration_ms),
    attempt.child?.tool_calls ?? null,
    attempt.child !== null && attempt.child.follow_ups > 0 ? attempt.child.follow_ups : null,
    kb(attempt.brief_bytes),
    kb(attempt.report_bytes),
    [attempt.decision, ...status].filter((value) => value !== null).join(', '),
    attempt.reason,
    attempt.native_child_id,
  ];
}

function renderTask(task: TaskRecord): string[] {
  const lines = [`### ${task.task_id}: ${task.title ?? 'untitled'}`, ''];
  lines.push(
    `State ${task.state}; ${task.work_items.length} work items, ${task.attempts.length} attempts; criteria changes ${task.criteria_changes}. Task dir \`${task.task_dir}\`.`,
  );
  for (const session of task.sessions) {
    lines.push(
      `Session ${session.index + 1}: ${session.harness ?? 'unknown harness'}, ${session.model ?? 'unknown model'}${session.effort === null ? '' : ` (${session.effort})`}, revision ${session.skill_revision}, outcome ${session.outcome ?? 'none (in progress or interrupted)'}${session.open_work_items.length > 0 ? `, open work: ${session.open_work_items.join('; ')}` : ''}. Parent transcript ${session.parent_transcript_path === null ? 'unavailable' : `\`${session.parent_transcript_path}\``}.`,
    );
  }
  for (const { conversation_id, scope, stats } of task.conversations) {
    const shell = Object.fromEntries(shellKinds.map((kind) => [kind, stats.shell[kind]]));
    const edits = Object.entries(stats.edits)
      .filter(([, edit]) => edit.calls > 0)
      .map(([scope, edit]) => `${scope} ${edit.calls} (${kb(edit.bytes)} KB)`)
      .join(', ');
    lines.push(
      `Parent ${conversation_id}${scope === 'conversation' ? ' (whole conversation)' : ''}: ${stats.tool_calls} tool calls (${counts(stats.tools)}); ${stats.dispatches} dispatches; ${stats.user_messages} user messages, ${stats.notifications} child notifications. Shell: ${counts(shell)}. Edits: ${edits || '-'}. Reads: ${counts(stats.reads)}.`,
    );
  }
  lines.push(
    `Bytes: briefs ${kb(task.bytes.briefs)} KB, reports ${kb(task.bytes.reports)} KB, parent journal ${kb(task.bytes.journal_parent)} KB, hook journal ${kb(task.bytes.journal_hook)} KB.`,
    '',
  );
  if (task.attempts.length > 0) {
    lines.push(
      ...table(
        ['attempt', 'role', 'requested model', 's', 'calls', 'follow-ups', 'brief KB', 'report KB', 'decision', 'reason', 'child id'],
        task.attempts.map(attemptRow),
      ),
      '',
    );
  }
  return lines;
}

export function renderMarkdown(analysis: Analysis): string {
  const { tasks, gaps } = analysis;
  const attempts = tasks.flatMap((task) => task.attempts);
  const states = counts(Object.fromEntries(['completed', 'blocked', 'open'].map((state) => [state, tasks.filter((task) => task.state === state).length])));
  const lines = [
    '# Orchestrate journal analysis',
    '',
    `Tasks ${tasks.length} (${states}); sessions ${tasks.reduce((total, task) => total + task.sessions.length, 0)}; attempts ${attempts.length} over ${tasks.reduce((total, task) => total + task.work_items.length, 0)} work items; gaps ${gaps.length}.`,
    '',
    'Briefs and reports are `<task dir>/briefs/<attempt>.md` and `<task dir>/reports/<attempt>.md`. Cursor child transcripts are `<parent transcript dir>/subagents/<child id>.jsonl`. "first accepted" counts first attempts accepted out of first attempts reviewed; "follow-ups" counts children that received a query after their brief; "(known)" is how many attempts had the value, and "~" marks background durations estimated from launch to the child transcript\'s last write. When tasks share a parent conversation, each gets the tool calls that name its task directory, the calls that follow them, and the messages they answer, unless marked as the whole conversation; "user messages" exclude the queries the harness sends when a background child finishes.',
    '',
  ];
  for (const dimension of dimensions) {
    lines.push(
      `## Attempts by ${dimensionTitles[dimension]}`,
      '',
      ...table(groupHeader, Object.entries(analysis.aggregates[dimension]).map(([key, stats]) => groupRow(key, stats))),
      '',
    );
  }
  lines.push(
    '## Parent sessions by skill revision',
    '',
    ...table(
      ['revision', 'sessions', 'with transcript', 'tool calls', 'dispatches', 'user messages', 'child notifications', ...shellKinds.map((kind) => `shell ${kind}`), 'source edits', 'source edit KB', 'source reads'],
      Object.entries(analysis.parent_by_skill_revision).map(([revision, stats]) => [
        revision,
        stats.sessions,
        stats.conversations_with_transcript,
        stats.tool_calls,
        stats.dispatches,
        stats.user_messages,
        stats.notifications,
        ...shellKinds.map((kind) => stats.shell[kind]),
        stats.source_edits,
        kb(stats.source_edit_bytes),
        stats.source_reads,
      ]),
    ),
    '',
    '## Tasks',
    '',
    ...tasks.flatMap(renderTask),
    '## Gaps',
    '',
  );
  if (gaps.length === 0) lines.push('None.');
  for (const gap of gaps) lines.push(`- ${gap.task_id}${gap.attempt_id === null ? '' : ` ${gap.attempt_id}`}: ${gap.message}`);
  return `${lines.join('\n')}\n`;
}
