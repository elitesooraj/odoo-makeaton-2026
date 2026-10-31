import { ApiError, ApiResponse } from './types';

const API_BASE_URL = '/api/v1';

export class ApiException extends Error {
  code: string;
  details?: Record<string, string>;
  requestId?: string;
  status: number;

  constructor(status: number, errorData: ApiError['error']) {
    super(errorData.message || 'An unexpected error occurred');
    this.name = 'ApiException';
    this.status = status;
    this.code = errorData.code || 'INTERNAL';
    this.details = errorData.details;
    this.requestId = errorData.request_id;
  }
}

export function isOfflineError(err: unknown): boolean {
  if (err instanceof TypeError) return true;
  if (err instanceof ApiException) {
    if (err.code === 'BACKEND_OFFLINE') return true;
    if (err.status === 500 || err.status === 502 || err.status === 503 || err.status === 504) {
      return true;
    }
  }
  if (err instanceof Error) {
    const msg = err.message.toLowerCase();
    if (
      msg.includes('fetch') ||
      msg.includes('econnrefused') ||
      msg.includes('failed to fetch') ||
      msg.includes('network') ||
      msg.includes('proxy error')
    ) {
      return true;
    }
  }
  return false;
}

interface RequestOptions extends RequestInit {
  params?: Record<string, string | number | boolean | undefined | null>;
}

export async function request<T>(
  path: string,
  options: RequestOptions = {}
): Promise<{ data: T; meta?: ApiResponse<T>['meta'] }> {
  let url = `${API_BASE_URL}${path}`;

  if (options.params) {
    const searchParams = new URLSearchParams();
    Object.entries(options.params).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== '') {
        searchParams.append(key, String(value));
      }
    });
    const queryString = searchParams.toString();
    if (queryString) {
      url += (url.includes('?') ? '&' : '?') + queryString;
    }
  }

  const headers: HeadersInit = {
    Accept: 'application/json',
    ...(options.body ? { 'Content-Type': 'application/json' } : {}),
    ...options.headers,
  };

  const response = await fetch(url, {
    ...options,
    headers,
    credentials: 'include', // Mandated by Brightloop session cookie convention
  });

  if (response.status === 204) {
    return { data: null as unknown as T };
  }

  let body: any = null;
  let rawText = '';
  try {
    rawText = await response.text();
    try {
      body = JSON.parse(rawText);
    } catch {
      body = null;
    }
  } catch {
    body = null;
  }

  if (!response.ok) {
    const isProxyDown =
      response.status >= 500 &&
      (rawText.includes('ECONNREFUSED') || rawText.includes('proxy error') || !body);

    const errorBody: ApiError['error'] = body?.error || {
      code: isProxyDown
        ? 'BACKEND_OFFLINE'
        : response.status === 401
        ? 'UNAUTHENTICATED'
        : response.status === 403
        ? 'FORBIDDEN'
        : 'INTERNAL',
      message: isProxyDown
        ? 'Backend API is currently offline on port 8080'
        : body?.message || response.statusText || 'Request failed',
      request_id: '',
    };
    throw new ApiException(response.status, errorBody);
  }

  return {
    data: body.data,
    meta: body.meta,
  };
}

export const api = {
  get: <T>(path: string, params?: RequestOptions['params']) =>
    request<T>(path, { method: 'GET', params }),
  post: <T>(path: string, body?: any) =>
    request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }),
  patch: <T>(path: string, body?: any) =>
    request<T>(path, { method: 'PATCH', body: body ? JSON.stringify(body) : undefined }),
  put: <T>(path: string, body?: any) =>
    request<T>(path, { method: 'PUT', body: body ? JSON.stringify(body) : undefined }),
  del: <T>(path: string) =>
    request<T>(path, { method: 'DELETE' }),
};
