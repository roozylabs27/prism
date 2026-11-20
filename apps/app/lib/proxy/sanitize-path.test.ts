import { describe, expect, it } from 'vitest';
import { sanitizeProxyPath } from './sanitize-path';

describe('sanitizeProxyPath', () => {
  it('accepts normal API and inference resource paths', () => {
    expect(sanitizeProxyPath(undefined)).toEqual({ ok: true, subPath: '' });
    expect(sanitizeProxyPath([])).toEqual({ ok: true, subPath: '' });
    expect(sanitizeProxyPath(['providers', 'prov_123', 'models'])).toEqual({
      ok: true,
      subPath: 'providers/prov_123/models',
    });
    expect(sanitizeProxyPath(['models', 'gpt-4.1-mini'])).toEqual({
      ok: true,
      subPath: 'models/gpt-4.1-mini',
    });
  });

  it('rejects dot segments, encoded traversal, delimiters, and schemes', () => {
    expect(sanitizeProxyPath(['..', 'internal'])).toMatchObject({ ok: false });
    expect(sanitizeProxyPath(['%2e%2e', 'secret'])).toMatchObject({ ok: false });
    expect(sanitizeProxyPath(['%252e%252e', 'secret'])).toMatchObject({ ok: false });
    expect(sanitizeProxyPath(['users', 'a?admin=1'])).toMatchObject({ ok: false });
    expect(sanitizeProxyPath(['users', 'a#frag'])).toMatchObject({ ok: false });
    expect(sanitizeProxyPath(['users', 'a\\b'])).toMatchObject({ ok: false });
    expect(sanitizeProxyPath(['http:', '169.254.169.254'])).toMatchObject({ ok: false });
    expect(sanitizeProxyPath(['bad%ZZpct'])).toMatchObject({ ok: false });
  });
});
