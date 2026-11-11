import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { after } from 'node:test';

export const repoRoot = join(import.meta.dirname, '..', '..', '..');
export const skillDir = join(repoRoot, 'agents', '.agents', 'skills', 'orchestrate');

export function tempDir(): string {
  const dir = mkdtempSync(join(tmpdir(), 'orchestrate-journal-'));
  after(() => rmSync(dir, { recursive: true, force: true }));
  return dir;
}

export function write(path: string, contents: string): string {
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, contents);
  return path;
}

export const jsonl = (lines: unknown[]): string => lines.map((line) => `${JSON.stringify(line)}\n`).join('');

let counter = 0;

export function event(
  type: string,
  writer: 'parent' | 'hook',
  data: Record<string, unknown>,
  options: { task_id: string; attempt_id?: string; supersedes?: string; event_id?: string; occurred_at?: string },
): Record<string, unknown> {
  counter++;
  const attemptId = options.attempt_id ?? null;
  return {
    schema_version: 3,
    event_id: options.event_id ?? `e${counter}`,
    occurred_at: options.occurred_at ?? new Date(Date.UTC(2026, 10, 4, 12, 0, counter)).toISOString().replace('.000', ''),
    type,
    writer,
    task_id: options.task_id,
    work_item_id: attemptId === null ? null : attemptId.replace(/-[0-9]+$/, ''),
    attempt_id: attemptId,
    supersedes: options.supersedes ?? null,
    data,
  };
}

export const taskStarted = {
  title: 'Fixture task',
  outcome: 'o',
  acceptance_criteria: ['a'],
  derived_from: null,
};

export function sessionStarted(conversationId: string, revision: unknown = { commit: 'abcdef0123456789', dirty: false }) {
  return {
    model: 'parent-model',
    effort: null,
    harness: 'cursor',
    conversation_id: conversationId,
    skill_revision: revision,
    repository: 'git@example.com:repo.git',
    working_directory: '/work',
    branch: 'main',
    starting_commit: '0123',
    existing_changes: [],
    utc_offset: '-03:00',
  };
}

export function attemptFinished(childId: string, transcriptPath: string, overrides: Record<string, unknown> = {}) {
  return {
    role: 'implementer',
    native_child_id: childId,
    child_type: 'generalPurpose',
    requested_model: 'child-model',
    harness_status: 'success',
    error_message: null,
    background: false,
    started_at: '2026-11-04T12:00:00Z',
    duration_ms: 60000,
    parent_conversation_id: 'conv-1',
    child_transcript_path: transcriptPath,
    harness_version: '2026.10.22',
    ...overrides,
  };
}

export function review(decision: 'accepted' | 'correction_required', reason: string | null = null) {
  return {
    child_status: 'complete',
    decision,
    reason,
    rationale: 'r',
    correction: decision === 'accepted' ? null : 'fix it',
  };
}

export const sessionFinished = {
  outcome: 'completed',
  criteria_met: ['a'],
  criteria_unmet: [],
  open_work_items: [],
  next_steps: [],
  journal_gaps: [],
};

export function toolUse(name: string, input: Record<string, unknown>) {
  return { type: 'tool_use', name, input };
}

export function transcript(messages: { role: 'user' | 'assistant'; content: unknown[] }[]): string {
  return jsonl([
    ...messages.map(({ role, content }) => ({ role, message: { content } })),
    { type: 'turn_ended', status: 'success' },
  ]);
}
