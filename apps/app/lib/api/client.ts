import axios from 'axios';
import { parseApiError } from '@/lib/http/errors';
import { getTenantHeaders } from '@/stores/useTenantStore';

export const api = axios.create({
  baseURL: '/api',
  timeout: 30_000,
  headers: {
    'Content-Type': 'application/json',
  },
  withCredentials: true,
});

// Axios request interceptor to attach tenant headers (auth relies on HttpOnly cookie via proxy)
api.interceptors.request.use((config) => {
  const tenantHeaders = getTenantHeaders();
  for (const [key, value] of Object.entries(tenantHeaders)) {
    config.headers[key] = value;
  }

  return config;
});

// Axios response interceptor for unified error parsing
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      // The session is gone; send the user to sign in instead of leaving
      // every query on the page failing. Sign-in itself answers 401 for a
      // wrong password, so stay put on the auth pages.
      if (
        typeof window !== 'undefined' &&
        !['/signin', '/signup', '/accept-invite'].includes(window.location.pathname)
      ) {
        window.location.assign('/signin');
      }
    }
    const apiError = parseApiError(error);
    return Promise.reject(apiError);
  }
);
