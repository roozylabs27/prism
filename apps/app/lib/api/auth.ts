import { api, resolveSignal, ApiSignal } from './client';
import {
  User,
  LoginRequest,
  SignupRequest,
  LoginResponse,
  ApiUserPermissionsResponse,
  ApiCompleteOnboardingRequest,
  ApiCompleteOnboardingResponse,
} from './types/auth';

export * from './types/auth';

export async function apiLogin(data: LoginRequest): Promise<LoginResponse> {
  const response = await api.post<LoginResponse>('/auth/login', data);
  return response.data;
}

export async function apiSignup(data: SignupRequest): Promise<LoginResponse> {
  const response = await api.post<LoginResponse>('/auth/signup', data);
  return response.data;
}

export async function apiLogout(): Promise<{ message: string }> {
  const response = await api.post<{ message: string }>('/auth/logout');
  return response.data;
}

export async function apiGetMe(signal?: ApiSignal): Promise<User> {
  const response = await api.get<User>('/auth/me', {
    signal: resolveSignal(signal),
  });
  return response.data;
}

export async function apiGetUserPermissions(signal?: ApiSignal): Promise<ApiUserPermissionsResponse> {
  const response = await api.get<ApiUserPermissionsResponse>('/user/permissions', {
    signal: resolveSignal(signal),
  });
  return response.data;
}

export async function apiCompleteOnboarding(data: ApiCompleteOnboardingRequest): Promise<ApiCompleteOnboardingResponse> {
  const response = await api.post<ApiCompleteOnboardingResponse>('/onboarding', data);
  return response.data;
}

export async function apiGetTurnstileConfig(signal?: AbortSignal): Promise<{ enabled?: boolean; siteKey: string }> {
  try {
    const response = await api.get<{ enabled?: boolean; siteKey: string }>('/auth/turnstile-config', { signal });
    return response.data;
  } catch {
    return {
      enabled: Boolean(process.env.NEXT_PUBLIC_CLOUDFLARE_SITE_KEY),
      siteKey: process.env.NEXT_PUBLIC_CLOUDFLARE_SITE_KEY || '1x00000000000000000000AA',
    };
  }
}
