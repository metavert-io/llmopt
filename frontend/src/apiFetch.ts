// Authenticated fetch wrapper — injects JWT + tenant headers on every API call.

import { getAccessToken, getActiveTenant, clearAuth } from './auth';

export async function apiFetch(
  url: string,
  options: RequestInit = {}
): Promise<Response> {
  const token = getAccessToken();
  const tenantId = getActiveTenant();

  const headers = new Headers(options.headers);
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  if (tenantId) {
    headers.set('X-Tenant-ID', tenantId);
  }

  const resp = await fetch(url, { ...options, headers });

  // On 401 with an existing token, the token is invalid/expired — clear and redirect.
  // If there's no token, the user was never logged in — don't redirect.
  if (resp.status === 401 && token) {
    clearAuth();
  }

  return resp;
}

// Statuses the proxy returns while the server is still starting up. The server
// sleeps when idle and wakes on the next request, so a request can arrive first.
const WAKING_STATUSES = new Set([502, 503, 504]);

function sleep(ms: number, signal?: AbortSignal | null): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(new DOMException('Aborted', 'AbortError'));
      return;
    }
    const timer = setTimeout(() => {
      signal?.removeEventListener('abort', onAbort);
      resolve();
    }, ms);
    const onAbort = () => {
      clearTimeout(timer);
      reject(new DOMException('Aborted', 'AbortError'));
    };
    signal?.addEventListener('abort', onAbort, { once: true });
  });
}

/**
 * apiFetch for starting a job, tolerant of a sleeping server: on 502/503/504 or
 * a network error it waits and retries for up to `maxWaitMs`, calling
 * `onWaiting` once so the page can say what's happening. The request body must
 * be a string (it is sent again on retry).
 */
export async function apiFetchWithWake(
  url: string,
  options: RequestInit,
  onWaiting?: () => void,
  maxWaitMs = 60000
): Promise<Response> {
  const started = Date.now();
  let delay = 1500;
  let notified = false;
  for (;;) {
    let resp: Response | null = null;
    try {
      resp = await apiFetch(url, options);
      if (!WAKING_STATUSES.has(resp.status)) return resp;
    } catch (err) {
      if (options.signal?.aborted || Date.now() - started >= maxWaitMs) throw err;
    }
    if (Date.now() - started >= maxWaitMs) {
      if (resp) return resp;
      throw new Error('The server did not respond. Try again in a moment.');
    }
    if (!notified) {
      notified = true;
      onWaiting?.();
    }
    await sleep(delay, options.signal);
    delay = Math.min(delay * 1.5, 5000);
  }
}

/** A plain sentence explaining why starting `what` (e.g. "test") failed. */
export async function describeHttpError(resp: Response, what: string): Promise<string> {
  let detail = '';
  try {
    const text = await resp.text();
    try {
      const j = JSON.parse(text);
      detail = j.message || j.error || '';
    } catch {
      detail = text.trim().slice(0, 200);
    }
  } catch {
    /* no body */
  }
  switch (resp.status) {
    case 429: {
      const retry = resp.headers.get('Retry-After');
      return `Too many ${what}s were started in the last minute. ${retry ? `Try again in ${retry} seconds.` : 'Try again shortly.'}`;
    }
    case 401:
      return 'Your session has expired. Sign in again, then retry.';
    case 403:
      return detail ? `Not allowed: ${detail}` : `Your account can't run this ${what}.`;
    case 502:
    case 503:
    case 504:
      return "The server didn't finish starting in time. Try again in a moment.";
  }
  return `Couldn't start the ${what} (HTTP ${resp.status})${detail ? `: ${detail}` : ''}.`;
}
