import { describe, expect, it } from 'vitest';
import { resourceSchema } from './create-resource.schema';

describe('create-resource schema', () => {
  it('validates parametersSchema JSON and backends array', () => {
    const parsed = resourceSchema.parse({
      name: 'docs_lookup',
      parametersSchema: '{"type":"object"}',
      backends: [
        {
          name: 'primary',
          backendType: 'http',
          endpointUrl: 'https://api.example.com/query',
        },
      ],
    });
    expect(parsed.name).toBe('docs_lookup');
    expect(parsed.backends).toHaveLength(1);
    expect(parsed.backends[0].backendType).toBe('http');
  });

  it('rejects invalid parametersSchema JSON', () => {
    const res = resourceSchema.safeParse({
      name: 'broken',
      parametersSchema: '{invalid',
    });
    expect(res.success).toBe(false);
  });
});
