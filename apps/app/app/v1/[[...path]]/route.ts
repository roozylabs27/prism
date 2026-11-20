import { NextRequest, NextResponse } from 'next/server';
import { sanitizeProxyPath } from '@/lib/proxy/sanitize-path';

export const dynamic = 'force-dynamic';

function getBackendApiUrl(): string {
  if (process.env.API_URL) {
    return process.env.API_URL.replace(/\/+$/, '');
  }
  if (process.env.NODE_ENV === 'development') {
    return 'http://localhost:8080';
  }
  return 'http://api:8080';
}

async function proxyV1Handler(request: NextRequest, { params }: { params: Promise<{ path?: string[] }> }) {
  const resolvedParams = await params;
  const sanitized = sanitizeProxyPath(resolvedParams?.path);
  if (!sanitized.ok) {
    return NextResponse.json(
      { error: { message: 'Invalid request path', type: 'invalid_request_error', code: 'invalid_path' } },
      { status: 400 }
    );
  }
  const subPath = sanitized.subPath;
  const searchParams = request.nextUrl.search;

  const backendApiUrl = getBackendApiUrl();
  const targetUrl = subPath ? `${backendApiUrl}/v1/${subPath}${searchParams}` : `${backendApiUrl}/v1${searchParams}`;

  const headers = new Headers(request.headers);
  headers.delete('host');

  let body: ArrayBuffer | undefined = undefined;
  if (['POST', 'PUT', 'PATCH', 'DELETE'].includes(request.method)) {
    try {
      body = await request.arrayBuffer();
    } catch (_bodyErr) {
      body = undefined;
    }
  }

  try {
    const backendResponse = await fetch(targetUrl, {
      method: request.method,
      headers: headers,
      body: body && body.byteLength > 0 ? body : undefined,
      cache: 'no-store',
      redirect: 'manual',
      signal: request.signal,
    });

    const responseHeaders = new Headers(backendResponse.headers);
    responseHeaders.delete('content-encoding');
    responseHeaders.delete('content-length');
    responseHeaders.delete('transfer-encoding');

    const contentType = responseHeaders.get('content-type') || '';
    const isSseContentType = contentType.includes('text/event-stream');
    const isNdjson = contentType.includes('application/x-ndjson');

    if (backendResponse.ok && (isSseContentType || isNdjson)) {
      if (isSseContentType) {
        responseHeaders.set('Content-Type', 'text/event-stream');
      }
      responseHeaders.set('Cache-Control', 'no-cache, no-transform');
      responseHeaders.set('Connection', 'keep-alive');
      responseHeaders.set('X-Accel-Buffering', 'no');

      return new NextResponse(backendResponse.body, {
        status: backendResponse.status,
        statusText: backendResponse.statusText,
        headers: responseHeaders,
      });
    }

    if (backendResponse.status === 204 || backendResponse.status === 304) {
      return new NextResponse(null, {
        status: backendResponse.status,
        statusText: backendResponse.statusText,
        headers: responseHeaders,
      });
    }

    const responseData = await backendResponse.arrayBuffer();
    return new NextResponse(responseData, {
      status: backendResponse.status,
      statusText: backendResponse.statusText,
      headers: responseHeaders,
    });
  } catch (error: unknown) {
    // The backend address and the connection error stay in the server log;
    // clients get an OpenAI-style error like any other gateway failure.
    console.error('[V1 Proxy Error] Failed to proxy to:', targetUrl, error);
    return NextResponse.json(
      { error: { message: 'The gateway is unavailable. Please try again shortly.', type: 'api_error', code: 'gateway_unavailable' } },
      { status: 502 }
    );
  }
}

export const GET = proxyV1Handler;
export const POST = proxyV1Handler;
export const PUT = proxyV1Handler;
export const PATCH = proxyV1Handler;
export const DELETE = proxyV1Handler;
export const OPTIONS = proxyV1Handler;
