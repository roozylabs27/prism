'use client';

import React, { createContext, useContext } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import Cookies from 'js-cookie';
import { AppRoutes, CookieKeys } from '@/constants/routes';
import { useTenantStore } from '@/stores/useTenantStore';
import { User, LoginRequest, SignupRequest, apiLogin, apiSignup, apiLogout, apiGetMe } from '@/lib/api';

interface AuthContextType {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (credentials: LoginRequest) => Promise<void>;
  signup: (payload: SignupRequest) => Promise<void>;
  logout: (redirectTo?: string) => Promise<void>;
}

const AuthContext = createContext<AuthContextType>({
  user: null,
  isAuthenticated: false,
  isLoading: true,
  login: async () => {},
  signup: async () => {},
  logout: async () => {},
});

export const useAuth = () => useContext(AuthContext);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const queryClient = useQueryClient();

  const {
    data: user = null,
    isLoading,
    isError,
  } = useQuery({
    queryKey: ['auth', 'me'],
    queryFn: apiGetMe,
    retry: false,
    staleTime: 5 * 60 * 1000,
  });

  const loginMutation = useMutation({
    mutationFn: apiLogin,
    onSuccess: (data) => {
      if (data.token) {
        Cookies.set('auth_token', data.token, { expires: 7, path: '/', sameSite: 'lax' });
      }
      queryClient.setQueryData(['auth', 'me'], data.user);
      queryClient.invalidateQueries({ queryKey: ['auth', 'me'] });
    },
  });

  const signupMutation = useMutation({
    mutationFn: apiSignup,
    onSuccess: (data) => {
      if (data.token) {
        Cookies.set('auth_token', data.token, { expires: 7, path: '/', sameSite: 'lax' });
      }
      queryClient.setQueryData(['auth', 'me'], data.user);
      queryClient.invalidateQueries({ queryKey: ['auth', 'me'] });
    },
  });

  const login = async (credentials: LoginRequest) => {
    await loginMutation.mutateAsync(credentials);
  };

  const signup = async (payload: SignupRequest) => {
    await signupMutation.mutateAsync(payload);
  };

  const logout = async (redirectTo?: string) => {
    try {
      await apiLogout();
    } catch {
      // Ignore network errors or already expired tokens on logout
    } finally {
      Cookies.remove(CookieKeys.AUTH_TOKEN, { path: '/' });
      Cookies.remove(CookieKeys.AUTH_TOKEN);
      useTenantStore.getState().resetTenant();
      queryClient.clear();
      const target =
        redirectTo && redirectTo.startsWith('/') && !redirectTo.startsWith('//')
          ? redirectTo
          : AppRoutes.SIGNIN;
      window.location.href = target;
    }
  };

  const isAuthenticated = !!user && !isError;

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated,
        isLoading,
        login,
        signup,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}
