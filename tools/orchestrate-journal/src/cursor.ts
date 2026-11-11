import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

export interface ToolUse {
  name: string;
  input: Record<string, unknown>;
}

// Transcript lines in order. User lines without a <user_query> are context the
// harness injects, such as attached skills, and are dropped.
export type Entry =
  | { kind: 'query' }
  | { kind: 'notification' }
  | { kind: 'tool'; use: ToolUse };

export interface Transcript {
  path: string;
  entries: Entry[];
  assistant_messages: number;
  turn_status: string | null;
  malformed_lines: number;
}

export interface ChildStats {
  tool_calls: number;
  tools: Record<string, number>;
  assistant_messages: number;
  // Queries after the brief: a resumed child or a follow-up message.
  follow_ups: number;
  turn_status: string | null;
}

export const shellKinds = ['journal', 'verification', 'task_artifacts', 'git_gh', 'exploration', 'other'] as const;
export type ShellKind = (typeof shellKinds)[number];

export const pathScopes = ['task', 'scratch', 'source'] as const;
export type PathScope = (typeof pathScopes)[number];

export interface ParentStats {
  tool_calls: number;
  tools: Record<string, number>;
  dispatches: number;
  user_messages: number;
  // Queries the harness sends when a background child finishes.
  notifications: number;
  shell: Record<ShellKind, number>;
  edits: Record<PathScope, { calls: number; bytes: number }>;
  reads: Record<PathScope, number>;
}

const isObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

// Cursor's wording for the query that follows a background child's completion.
const notificationQuery =
  /<user_query>\s*(Perform any necessary follow-up actions in response to the subagent completion|Briefly inform the user about the task result)/;

function messageText(message: unknown): string {
  const content = isObject(message) ? message.content : undefined;
  if (typeof content === 'string') return content;
  if (!Array.isArray(content)) return '';
  return content.map((block) => (isObject(block) && typeof block.text === 'string' ? block.text : '')).join('\n');
}

export function readTranscript(path: string): Transcript {
  const transcript: Transcript = {
    path,
    entries: [],
    assistant_messages: 0,
    turn_status: null,
    malformed_lines: 0,
  };
  for (const raw of readFileSync(path, 'utf8').split('\n')) {
    if (raw.trim() === '') continue;
    let line: unknown;
    try {
      line = JSON.parse(raw);
    } catch {
      transcript.malformed_lines++;
      continue;
    }
    if (!isObject(line)) {
      transcript.malformed_lines++;
      continue;
    }
    if (line.type === 'turn_ended') {
      transcript.turn_status = typeof line.status === 'string' ? line.status : null;
      continue;
    }
    if (line.role === 'user') {
      const text = messageText(line.message);
      if (notificationQuery.test(text)) transcript.entries.push({ kind: 'notification' });
      else if (text.includes('<user_query>')) transcript.entries.push({ kind: 'query' });
      continue;
    }
    if (line.role !== 'assistant') continue;
    transcript.assistant_messages++;
    const content = isObject(line.message) && Array.isArray(line.message.content) ? line.message.content : [];
    for (const block of content) {
      if (!isObject(block) || block.type !== 'tool_use' || typeof block.name !== 'string') continue;
      transcript.entries.push({ kind: 'tool', use: unwrapToolUse(block.name, isObject(block.input) ? block.input : {}) });
    }
  }
  return transcript;
}

const toolUses = (entries: readonly Entry[]): ToolUse[] => entries.flatMap((entry) => (entry.kind === 'tool' ? [entry.use] : []));
const countKind = (entries: readonly Entry[], kind: Entry['kind']): number => entries.filter((entry) => entry.kind === kind).length;

// Native Cursor tools such as Task may be invoked through CallDynamicTool.
function unwrapToolUse(name: string, input: Record<string, unknown>): ToolUse {
  if (name !== 'CallDynamicTool' || typeof input.toolName !== 'string') return { name, input };
  const inner = isObject(input.arguments) ? input.arguments : {};
  const qualified = input.namespace === 'cursor' ? input.toolName : `${String(input.namespace)}.${input.toolName}`;
  return { name: qualified, input: inner };
}

function countTools(uses: readonly ToolUse[]): Record<string, number> {
  const tools: Record<string, number> = {};
  for (const use of uses) tools[use.name] = (tools[use.name] ?? 0) + 1;
  return tools;
}

export function childStats(transcript: Transcript): ChildStats {
  const uses = toolUses(transcript.entries);
  return {
    tool_calls: uses.length,
    tools: countTools(uses),
    assistant_messages: transcript.assistant_messages,
    follow_ups: Math.max(0, countKind(transcript.entries, 'query') - 1),
    turn_status: transcript.turn_status,
  };
}

const verificationPrograms = new Set([
  'tsc',
  'jest',
  'vitest',
  'mocha',
  'pytest',
  'eslint',
  'oxlint',
  'biome',
  'prettier',
  'oxfmt',
  'gofmt',
  'golangci-lint',
  'shellcheck',
]);
const verificationTarget = /test|lint|typecheck|type-check|format|fmt|check|sync/;
const explorationPrograms = new Set(['rg', 'grep', 'fd', 'find', 'cat', 'sed', 'awk', 'ls', 'head', 'tail', 'jq', 'wc', 'tree', 'less', 'bat', 'diff']);
const packageRunners = new Set(['yarn', 'npx', 'npm', 'pnpm', 'bun', 'bunx']);

interface Invocation {
  program: string;
  first: string;
  // True when the program is a package script or binary run through yarn, npx, and similar.
  script: boolean;
}

// Skips environment assignments and package runner verbs such as `npm run`.
function invocation(segment: string): Invocation {
  const words = segment.trim().split(/\s+/).filter((word) => !/^[A-Za-z_][A-Za-z0-9_]*=/.test(word));
  let [program = '', ...args] = words;
  program = program.replace(/^.*\//, '');
  const script = packageRunners.has(program);
  if (script) {
    while (args[0] !== undefined && ['run', 'exec', '--', '-s', '--silent'].includes(args[0])) args.shift();
    program = args.shift() ?? '';
  }
  return { program, first: args[0] ?? '', script };
}

function classifySegment(segment: string): ShellKind {
  const { program, first, script } = invocation(segment);
  if (verificationPrograms.has(program)) return 'verification';
  if (program === 'go' && ['test', 'vet'].includes(first)) return 'verification';
  if ((program === 'make' || program === 'nx') && verificationTarget.test(first)) return 'verification';
  if (script && verificationTarget.test(program)) return 'verification';
  if (program === 'git' || program === 'gh') return 'git_gh';
  if (explorationPrograms.has(program)) return 'exploration';
  return 'other';
}

const kindPriority: readonly ShellKind[] = ['verification', 'git_gh', 'exploration', 'other'];
const heredocBody = /<<-?\s*(['"]?)(\w+)\1([^\n]*)\n[\s\S]*?\n\s*\2(?=\n|$)/g;

export function classifyShell(command: string): ShellKind {
  if (/journal\.jsonl|journal-append/.test(command)) return 'journal';
  const code = command.replace(heredocBody, '$3');
  const kinds = new Set(code.split(/&&|\|\||[;|\n(){}`$]/).filter((part) => part.trim() !== '').map(classifySegment));
  if (kinds.has('verification')) return 'verification';
  if (/\.scratch\//.test(code)) return 'task_artifacts';
  return kindPriority.find((kind) => kinds.has(kind)) ?? 'other';
}

export function pathScope(path: string, taskDir: string): PathScope {
  if (path === taskDir || path.startsWith(`${taskDir}/`)) return 'task';
  if (/(^|\/)\.scratch\//.test(path)) return 'scratch';
  return 'source';
}

const editTools: Readonly<Record<string, string>> = {
  Write: 'contents',
  StrReplace: 'new_string',
  EditNotebook: 'new_string',
  Delete: '',
};

export function parentStats(entries: readonly Entry[], taskDir: string): ParentStats {
  const uses = toolUses(entries);
  const stats: ParentStats = {
    tool_calls: uses.length,
    tools: countTools(uses),
    dispatches: 0,
    user_messages: countKind(entries, 'query'),
    notifications: countKind(entries, 'notification'),
    shell: Object.fromEntries(shellKinds.map((kind) => [kind, 0])) as Record<ShellKind, number>,
    edits: Object.fromEntries(pathScopes.map((scope) => [scope, { calls: 0, bytes: 0 }])) as ParentStats['edits'],
    reads: Object.fromEntries(pathScopes.map((scope) => [scope, 0])) as Record<PathScope, number>,
  };
  for (const { name, input } of uses) {
    const path = typeof input.path === 'string' ? input.path : typeof input.target_notebook === 'string' ? input.target_notebook : null;
    if (name === 'Task') stats.dispatches++;
    else if (name === 'Shell' && typeof input.command === 'string') stats.shell[classifyShell(input.command)]++;
    else if (name === 'Read' && path !== null) stats.reads[pathScope(path, taskDir)]++;
    else if (name in editTools && path !== null) {
      const edit = stats.edits[pathScope(path, taskDir)];
      const content = input[editTools[name] ?? ''];
      edit.calls++;
      edit.bytes += typeof content === 'string' ? Buffer.byteLength(content, 'utf8') : 0;
    }
  }
  return stats;
}

export interface Dispatch {
  child_type: string | null;
  requested_model: string | null;
  // Task runs in the foreground unless run_in_background is true.
  background: boolean;
}

// The last Task call that dispatched the attempt. A prompt's Report: line
// decides when present, since full briefs can link earlier attempts' briefs;
// otherwise the prompt must point to the attempt's brief.
export function findDispatch(entries: readonly Entry[], taskId: string, attemptId: string): Dispatch | null {
  const taskPath = `.scratch/${taskId}/`;
  let found: Dispatch | null = null;
  for (const entry of entries) {
    if (entry.kind !== 'tool' || entry.use.name !== 'Task') continue;
    const { prompt, model, subagent_type, run_in_background } = entry.use.input;
    if (typeof prompt !== 'string') continue;
    const report = /^Report: (.+)$/m.exec(prompt)?.[1]?.trim();
    const named =
      report === undefined ? prompt.includes(`${taskPath}briefs/${attemptId}.md`) : report.endsWith(`${taskPath}reports/${attemptId}.md`);
    if (!named) continue;
    found = {
      child_type: typeof subagent_type === 'string' ? subagent_type : null,
      requested_model: typeof model === 'string' ? model : null,
      background: run_in_background === true,
    };
  }
  return found;
}

// The task whose directory the call targets: a file path, a brief's Report:
// line, or the last task directory a shell command names outside heredoc
// bodies, whose data can name other tasks, for example in derived_from.
function namedTask(use: ToolUse, taskIds: readonly string[]): string | null {
  const { command, path, prompt } = use.input;
  const text =
    use.name === 'Shell' && typeof command === 'string'
      ? command.replace(heredocBody, '$3')
      : typeof path === 'string'
        ? path
        : typeof prompt === 'string'
          ? (/^Report: (.+)$/m.exec(prompt)?.[1] ?? prompt)
          : JSON.stringify(use.input);
  let named: string | null = null;
  let at = -1;
  for (const taskId of taskIds) {
    for (const match of text.matchAll(new RegExp(`\\.scratch/${RegExp.escape(taskId)}(?![\\w-])`, 'g'))) {
      if (match.index > at) [named, at] = [taskId, match.index];
    }
  }
  return named;
}

// Splits a conversation that serves several tasks. Calls that name a task's
// directory belong to it, other calls to the latest named task, and messages
// to the next one, since the parent's reply acts on them. Returns null when a
// task is never named, so the split would be unreliable.
export function attributeEntries(entries: readonly Entry[], taskIds: readonly string[]): Map<string, Entry[]> | null {
  const owners = entries.map((entry) => (entry.kind === 'tool' ? namedTask(entry.use, taskIds) : null));
  let latest: string | null = null;
  entries.forEach((entry, index) => {
    if (entry.kind === 'tool') latest = owners[index] ??= latest;
  });
  let next: string | null = null;
  for (let index = entries.length - 1; index >= 0; index--) {
    if (owners[index] === null) owners[index] = next;
    else if (entries[index]?.kind === 'tool') next = owners[index] ?? null;
  }
  latest = null;
  owners.forEach((owner, index) => {
    latest = owners[index] = owner ?? latest;
  });
  const parts = new Map<string, Entry[]>(taskIds.map((taskId) => [taskId, []]));
  entries.forEach((entry, index) => {
    const owner = owners[index];
    if (owner != null) parts.get(owner)?.push(entry);
  });
  return taskIds.every((taskId) => owners.includes(taskId)) ? parts : null;
}

export function findParentTranscript(cursorDir: string, conversationId: string): string | null {
  const projects = join(cursorDir, 'projects');
  if (!existsSync(projects)) return null;
  for (const project of readdirSync(projects)) {
    const candidate = join(projects, project, 'agent-transcripts', conversationId, `${conversationId}.jsonl`);
    if (existsSync(candidate)) return candidate;
  }
  return null;
}
