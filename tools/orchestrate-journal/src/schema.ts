// The journal contract lives in the skill: journal-append and the Cursor hook
// write through scripts/journal-schema.json, and references/activity-journal.md
// describes it.
import contract from '../../../agents/.agents/skills/orchestrate/scripts/journal-schema.json' with { type: 'json' };

export const schemaVersion: number = contract.schema_version;

export const envelopeKeys: readonly string[] = contract.envelope_keys;

export const attemptIdPattern = new RegExp(contract.attempt_id_pattern);

const fieldTypeNames = ['string', 'string?', 'strings', 'strings?', 'boolean?', 'count?', 'revision?'] as const;

export type FieldType = (typeof fieldTypeNames)[number] | readonly (string | null)[];

export interface EventSpec {
  writer: 'parent' | 'hook';
  scope: 'task' | 'attempt' | 'any';
  // Keys the writer passes to journal-append; the rest are filled by journal-append.
  inputKeys: readonly string[];
  fields: Readonly<Record<string, FieldType>>;
}

function oneOf<T extends string>(options: readonly T[], value: string, where: string): T {
  const match = options.find((option) => option === value);
  if (match === undefined) throw new Error(`journal-schema.json: ${where} is ${JSON.stringify(value)}`);
  return match;
}

function fieldTypes(fields: Record<string, string | (string | null)[]>, where: string): Record<string, FieldType> {
  return Object.fromEntries(
    Object.entries(fields).map(([key, type]) => [
      key,
      typeof type === 'string' ? oneOf(fieldTypeNames, type, `${where}.${key}`) : type,
    ]),
  );
}

export const eventSpecs: Readonly<Record<string, EventSpec>> = Object.fromEntries(
  Object.entries(contract.events).map(([type, spec]) => [
    type,
    {
      writer: oneOf(['parent', 'hook'], spec.writer, `${type}.writer`),
      scope: oneOf(['task', 'attempt', 'any'], spec.scope, `${type}.scope`),
      inputKeys: Object.keys(spec.fields),
      fields: {
        ...fieldTypes(spec.fields, type),
        ...('added_fields' in spec ? fieldTypes(spec.added_fields, type) : {}),
      },
    },
  ]),
);

const isStrings = (value: unknown): boolean =>
  Array.isArray(value) && value.every((item) => typeof item === 'string');

const isNonEmptyString = (value: unknown): boolean => typeof value === 'string' && value.length > 0;

export function isValid(value: unknown, type: FieldType): boolean {
  if (typeof type !== 'string') return type.includes(value as string | null);
  switch (type) {
    case 'string':
      return isNonEmptyString(value);
    case 'string?':
      return value === null || isNonEmptyString(value);
    case 'strings':
      return isStrings(value);
    case 'strings?':
      return value === null || isStrings(value);
    case 'boolean?':
      return value === null || typeof value === 'boolean';
    case 'count?':
      return value === null || (Number.isInteger(value) && (value as number) >= 0);
    case 'revision?': {
      if (value === null) return true;
      if (typeof value !== 'object' || Array.isArray(value)) return false;
      const revision = value as Record<string, unknown>;
      // Migrated journals keep a known commit with an unknown dirty state.
      return (
        Object.keys(revision).sort().join(',') === 'commit,dirty' &&
        isNonEmptyString(revision.commit) &&
        (revision.dirty === null || typeof revision.dirty === 'boolean')
      );
    }
  }
}

function describe(type: FieldType): string {
  if (typeof type !== 'string') return `one of ${type.map((value) => JSON.stringify(value)).join(', ')}`;
  return {
    string: 'a non-empty string',
    'string?': 'a non-empty string or null',
    strings: 'an array of strings',
    'strings?': 'an array of strings or null',
    'boolean?': 'a boolean or null',
    'count?': 'a non-negative integer or null',
    'revision?': 'null or {commit, dirty}',
  }[type];
}

export function dataProblems(spec: EventSpec, data: Record<string, unknown>): string[] {
  const expected = Object.keys(spec.fields);
  const actual = Object.keys(data);
  const problems = [
    ...expected.filter((key) => !actual.includes(key)).map((key) => `missing key: ${key}`),
    ...actual.filter((key) => !expected.includes(key)).map((key) => `unknown key: ${key}`),
  ];
  for (const [key, type] of Object.entries(spec.fields)) {
    if (key in data && !isValid(data[key], type)) problems.push(`${key} must be ${describe(type)}`);
  }
  return problems;
}
