import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

const SSR_PERMISSIONS_HEADER = 'x-prism-ssr-permissions';

function getBackendApiUrl(): string {
  if (process.env.API_URL) return process.env.API_URL;
  if (process.env.INTERNAL_API_URL) return process.env.INTERNAL_API_URL;
  if (process.env.NODE_ENV === 'development') return 'http://localhost:8080';
  return 'http://api:8080';
}

export async function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const token = request.cookies.get('auth_token')?.value;
  // Strip any client-supplied internal header before forwarding downstream.
  const requestHeaders = new Headers(request.headers);
  requestHeaders.delete(SSR_PERMISSIONS_HEADER);

  const isAuthPage = pathname === '/signin' || pathname === '/signup';
  const isOnboardingPage = pathname === '/onboarding';
  // An invitation link is opened by people without an account as well as by
  // signed-in users who have not been onboarded; the page handles both.
  const isInvitePage = pathname === '/accept-invite';
  const inviteToken = request.nextUrl.searchParams.get('invite');
  const isStaticAsset =
    pathname.startsWith('/_next') ||
    pathname.includes('favicon.ico') ||
    pathname.includes('favicon.svg');
  // Backend API and LLM inference reverse-proxies (/api/*, /v1/*) are authenticated
  // and authorized directly at the Go engine boundary (internal/middleware/auth.go).
  const isProxiedBackendRoute =
    pathname.startsWith('/api') ||
    pathname.startsWith('/v1');
  const isHealthCheck = pathname === '/health';

  if (isStaticAsset || isProxiedBackendRoute || isHealthCheck) {
    return NextResponse.next();
  }

  // Restrict protected pages if no auth_token cookie
  if (!token && !isAuthPage && !isInvitePage) {
    const signinUrl = new URL('/signin', request.url);
    return NextResponse.redirect(signinUrl);
  }
  if (isInvitePage) {
    return NextResponse.next({ request: { headers: requestHeaders } });
  }

  // Server-side onboarding & permissions check: forwards verified permissions to
  // DashboardLayout so RSC renders do not trigger a second /api/user/permissions call.
  if (token) {
    try {
      const backendUrl = getBackendApiUrl();
      const activeOrgHint = request.cookies.get('prism_active_org')?.value?.trim();
      const fetchHeaders: Record<string, string> = {
        Cookie: activeOrgHint
          ? `auth_token=${token}; prism_active_org=${encodeURIComponent(activeOrgHint)}`
          : `auth_token=${token}`,
        Authorization: `Bearer ${token}`,
      };
      if (activeOrgHint) {
        fetchHeaders['X-Prism-Org-ID'] = activeOrgHint;
      }

      const res = await fetch(`${backendUrl}/api/user/permissions`, {
        headers: fetchHeaders,
        cache: 'no-store',
      });

      if (res.status === 401) {
        // Session token expired or revoked: clear stale cookie so protected
        // pages never get stuck in a loop and auth pages can sign in cleanly.
        const out = !isAuthPage
          ? NextResponse.redirect(new URL('/signin', request.url))
          : NextResponse.next({ request: { headers: requestHeaders } });
        out.cookies.delete('auth_token');
        return out;
      }

      if (res.ok) {
        const data = await res.json();
        const isOnboarded = Boolean(data?.isOnboarded);

        // A signed-in user following an invitation link must be able to reach it
        // even before completing organization onboarding.
        if (isAuthPage && inviteToken) {
          const inviteUrl = new URL('/accept-invite', request.url);
          inviteUrl.searchParams.set('token', inviteToken);
          return NextResponse.redirect(inviteUrl);
        }

        // If user is not yet onboarded, redirect immediately on the server to /onboarding
        if (!isOnboarded && !isOnboardingPage) {
          const onboardingUrl = new URL('/onboarding', request.url);
          return NextResponse.redirect(onboardingUrl);
        }

        // If user is already onboarded, redirect away from /onboarding and /signin or /signup to /
        if (isOnboarded && (isOnboardingPage || isAuthPage)) {
          const dashboardUrl = new URL('/', request.url);
          return NextResponse.redirect(dashboardUrl);
        }

        requestHeaders.set(SSR_PERMISSIONS_HEADER, encodeURIComponent(JSON.stringify(data)));
        return NextResponse.next({ request: { headers: requestHeaders } });
      }
    } catch (_err) {
      // Do not redirect away from auth pages on network/fetch errors; otherwise
      // an unreachable or erroring backend causes an infinite /signin <-> / loop.
    }
  }

  return NextResponse.next({ request: { headers: requestHeaders } });
}

export const config = {
  matcher: ['/((?!_next/static|_next/image|favicon.ico|favicon.svg|api/).*)'],
};
