export type SanitizeProxyPathResult =
  | { ok: true; subPath: string }
  | { ok: false; reason: string };

const CONTROL_CHAR_REGEX = /[\u0000-\u001f\u007f]/;
const PROTOCOL_PREFIX_REGEX = /^[a-z][a-z0-9+.-]*:$/i;

function decodeSafe(segment: string): string | null {
  let current = segment;
  for (let pass = 0; pass < 2; pass++) {
    try {
      const next = decodeURIComponent(current);
      if (next === current) {
        return current;
      }
      current = next;
    } catch {
      return null;
    }
  }
  try {
    // Reject >2 levels of double/triple percent-encoding tricks such as %252e%252e.
    if (decodeURIComponent(current) !== current) {
      return null;
    }
  } catch {
    return null;
  }
  return current;
}

export function sanitizeProxyPath(segments?: string[]): SanitizeProxyPathResult {
  if (!segments || segments.length === 0) {
    return { ok: true, subPath: '' };
  }

  const cleanSegments: string[] = [];
  for (let i = 0; i < segments.length; i++) {
    const raw = segments[i];
    if (!raw || raw === '.' || raw === '..' || CONTROL_CHAR_REGEX.test(raw)) {
      return { ok: false, reason: 'Empty, dot, or control segment in path' };
    }
    if (raw.includes('/') || raw.includes('\\') || raw.includes('?') || raw.includes('#')) {
      return { ok: false, reason: 'Illegal delimiter in path segment' };
    }

    const decoded = decodeSafe(raw);
    if (decoded === null || !decoded) {
      return { ok: false, reason: 'Malformed encoding in path segment' };
    }
    if (
      decoded === '.' ||
      decoded === '..' ||
      decoded.includes('..') ||
      decoded.includes('/') ||
      decoded.includes('\\') ||
      decoded.includes('?') ||
      decoded.includes('#') ||
      CONTROL_CHAR_REGEX.test(decoded)
    ) {
      return { ok: false, reason: 'Path traversal or delimiter smuggling rejected' };
    }
    if (i === 0 && (PROTOCOL_PREFIX_REGEX.test(decoded) || PROTOCOL_PREFIX_REGEX.test(raw))) {
      return { ok: false, reason: 'Protocol scheme prefix is not allowed in proxy path' };
    }

    cleanSegments.push(raw);
  }

  return { ok: true, subPath: cleanSegments.join('/') };
}
