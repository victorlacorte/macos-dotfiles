#!/usr/bin/env node
import { homedir } from 'node:os';
import { join } from 'node:path';
import { parseArgs } from 'node:util';
import { analyze, InvalidJournalsError, type Analysis } from './analyze.ts';
import { renderMarkdown } from './render.ts';

const usage = `usage:
  orchestrate-journal analyze [--json] [--cursor-dir <dir>] <journal.jsonl>...

analyze  reports delegation and cost-proxy metrics for the given task journals.
         Fails without a report when any journal breaks the current schema.

Find journals with, for example: fd -H -I '^journal\\.jsonl$' ~/coding`;

function fail(message: string): never {
  process.stderr.write(`orchestrate-journal: ${message}\n${usage}\n`);
  process.exit(2);
}

function main(argv: string[]): number {
  const [command, ...rest] = argv;
  if (command === '-h' || command === '--help') {
    process.stdout.write(`${usage}\n`);
    return 0;
  }
  if (command !== 'analyze') fail(command === undefined ? 'missing command' : `unknown command: ${command}`);
  const { values, positionals } = parseArgs({
    args: rest,
    allowPositionals: true,
    options: { json: { type: 'boolean', default: false }, 'cursor-dir': { type: 'string' } },
  });
  if (positionals.length === 0) fail('analyze needs at least one journal path');
  let analysis: Analysis;
  try {
    analysis = analyze(positionals, { cursor_dir: values['cursor-dir'] ?? join(homedir(), '.cursor') });
  } catch (error) {
    if (!(error instanceof InvalidJournalsError)) throw error;
    process.stderr.write(`orchestrate-journal: ${error.message}\n`);
    return 1;
  }
  process.stdout.write(values.json ? `${JSON.stringify(analysis, null, 2)}\n` : renderMarkdown(analysis));
  return 0;
}

process.exitCode = main(process.argv.slice(2));
