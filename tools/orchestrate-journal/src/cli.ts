#!/usr/bin/env node
import { homedir } from 'node:os';
import { join } from 'node:path';
import { parseArgs } from 'node:util';
import { analyze } from './analyze.ts';
import { migrateFile } from './migrate.ts';
import { renderMarkdown } from './render.ts';

const usage = `usage:
  orchestrate-journal analyze [--json] [--cursor-dir <dir>] <journal.jsonl>...
  orchestrate-journal migrate [--write] <journal.jsonl>...

analyze  reports delegation and cost-proxy metrics for the given task journals.
migrate  lists rewrites from older journal shapes to the current schema; --write
         applies them in place and keeps the first original as journal.jsonl.orig.

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
  if (command === 'analyze') {
    const { values, positionals } = parseArgs({
      args: rest,
      allowPositionals: true,
      options: { json: { type: 'boolean', default: false }, 'cursor-dir': { type: 'string' } },
    });
    if (positionals.length === 0) fail('analyze needs at least one journal path');
    const analysis = analyze(positionals, { cursor_dir: values['cursor-dir'] ?? join(homedir(), '.cursor') });
    process.stdout.write(values.json ? `${JSON.stringify(analysis, null, 2)}\n` : renderMarkdown(analysis));
    return 0;
  }
  if (command === 'migrate') {
    const { values, positionals } = parseArgs({
      args: rest,
      allowPositionals: true,
      options: { write: { type: 'boolean', default: false } },
    });
    if (positionals.length === 0) fail('migrate needs at least one journal path');
    let failed = false;
    for (const path of positionals) {
      try {
        const migrations = migrateFile(path, values.write);
        const verb = values.write ? 'migrated' : 'would migrate';
        process.stdout.write(migrations.length === 0 ? `${path}: current\n` : `${path}: ${verb} ${migrations.length} lines\n`);
        for (const { line, changes } of migrations) process.stdout.write(`  line ${line}: ${changes.join('; ')}\n`);
      } catch (error) {
        failed = true;
        process.stderr.write(`orchestrate-journal: ${path}: ${(error as Error).message}\n`);
      }
    }
    return failed ? 1 : 0;
  }
  fail(command === undefined ? 'missing command' : `unknown command: ${command}`);
}

process.exitCode = main(process.argv.slice(2));
