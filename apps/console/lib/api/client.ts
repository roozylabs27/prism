import axios from 'axios';
import { parseApiError } from '@/lib/http/errors';
import { getTenantHeaders } from '@/stores/useTenantStore';

export type ApiSignal = AbortSignal | { signal?: AbortSignal };

export function resolveSignal(signal?: ApiSignal): AbortSignal | undefined {
  if (!signal) return undefined;
  if ('signal' in signal && (signal as { signal?: AbortSignal }).signal) {
    return (signal as { signal?: AbortSignal }).signal;
  }
  return signal as AbortSignal;
}

export const api = axios.create({
  baseURL: '/api',
  timeout: 30_000,
  headers: {
    'Content-Type': 'application/json',
  },
  withCredentials: true,
});

const ORG_DISCOVERY_PATHS = new Set([
  'user/permissions',
  'user/organizations',
  'organizations',
  'workspaces',
]);

function isOrgDiscoveryRequest(url?: string): boolean {
  if (!url) return false;
  const clean = url.split('?')[0].replace(/^\/+/, '');
  return ORG_DISCOVERY_PATHS.has(clean);
}

// Axios request interceptor to attach tenant headers (auth relies on HttpOnly cookie via proxy)
api.interceptors.request.use((config) => {
  const tenantHeaders = getTenantHeaders();
  const orgDiscoveryOnly = isOrgDiscoveryRequest(config.url);
  for (const [key, value] of Object.entries(tenantHeaders)) {
    // Org/workspace discovery and permission checks must stay org-scoped so selecting
    // an inaccessible workspace cannot lock the user out of recovery.
    if (orgDiscoveryOnly && (key === 'X-Prism-Workspace-ID' || key === 'X-Prism-Project-ID')) {
      continue;
    }
    config.headers[key] = value;
  }

  return config;
});

function isBlobLike(val: unknown): val is Blob {
  return (
    (typeof Blob !== 'undefined' && val instanceof Blob) ||
    Object.prototype.toString.call(val) === '[object Blob]'
  );
}

async function readBlobText(blob: Blob): Promise<string> {
  if (typeof blob.text === 'function') {
    return blob.text();
  }
  if (typeof FileReader !== 'undefined') {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '');
      reader.onerror = () => reject(reader.error);
      reader.readAsText(blob);
    });
  }
  if (typeof Response !== 'undefined') {
    return new Response(blob).text();
  }
  return '';
}

// Axios response interceptor for unified error parsing
api.interceptors.response.use(
  (response) => response,
  async (error) => {
    if (error.response?.status === 401) {
      // The session is gone; send the user to sign in instead of leaving
      // every query on the page failing. Sign-in itself answers 401 for a
      // wrong password, so stay put on the auth pages.
      if (
        typeof window !== 'undefined' &&
        !['/signin', '/signup', '/accept-invite'].includes(window.location.pathname)
      ) {
        // eslint-disable-next-line @next/next/no-location-assign-relative-destination -- Intentional full navigation outside React tree on 401 session expiry
        window.location.assign('/signin');
      }
    }
    if (isBlobLike(error?.response?.data)) {
      try {
        const text = await readBlobText(error.response.data);
        if (text) {
          try {
            error.response.data = JSON.parse(text);
          } catch {
            error.response.data = text;
          }
        }
      } catch {
        // Keep original blob if reading fails
      }
    }
    const apiError = parseApiError(error);
    return Promise.reject(apiError);
  }
);
