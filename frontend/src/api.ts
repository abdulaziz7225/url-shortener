export const API_BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:8080";

export interface CreatePayload {
  long_url: string;
  custom_alias?: string;
  expires_at?: string;
}

export interface UrlRecord {
  code: string;
  short_url: string;
  long_url: string;
  expires_at?: string;
}

interface ApiError {
  error: { code: string; message: string };
}

export async function createShortUrl(payload: CreatePayload): Promise<UrlRecord> {
  const res = await fetch(`${API_BASE}/api/v1/urls`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const data: unknown = await res.json();
  if (!res.ok) {
    const message = (data as ApiError).error?.message ?? `request failed (${res.status})`;
    throw new Error(message);
  }
  return data as UrlRecord;
}

export type LinkStatus = "active" | "expired" | "missing" | "unknown";

export async function checkStatus(code: string): Promise<LinkStatus> {
  const res = await fetch(`${API_BASE}/api/v1/urls/${encodeURIComponent(code)}`);
  switch (res.status) {
    case 200:
      return "active";
    case 410:
      return "expired";
    case 404:
      return "missing";
    default:
      return "unknown";
  }
}
