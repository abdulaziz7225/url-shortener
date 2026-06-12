import { useEffect, useState } from "react";
import {
  API_BASE,
  checkStatus,
  createShortUrl,
  type LinkStatus,
  type UrlRecord,
} from "./api";

const STORAGE_KEY = "url-shortener.links";

function loadLinks(): UrlRecord[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? (JSON.parse(raw) as UrlRecord[]) : [];
  } catch {
    return [];
  }
}

export function App() {
  const [longUrl, setLongUrl] = useState("");
  const [alias, setAlias] = useState("");
  const [expiresAt, setExpiresAt] = useState("");
  const [links, setLinks] = useState<UrlRecord[]>(loadLinks);
  const [statuses, setStatuses] = useState<Record<string, LinkStatus>>({});
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(links));
  }, [links]);

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    setError("");
    setSubmitting(true);
    try {
      const payload = {
        long_url: longUrl.trim(),
        ...(alias.trim() ? { custom_alias: alias.trim() } : {}),
        ...(expiresAt ? { expires_at: new Date(expiresAt).toISOString() } : {}),
      };
      const record = await createShortUrl(payload);
      setLinks((prev) => [record, ...prev.filter((l) => l.code !== record.code)]);
      setStatuses((prev) => ({ ...prev, [record.code]: "active" }));
      setLongUrl("");
      setAlias("");
      setExpiresAt("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "something went wrong");
    } finally {
      setSubmitting(false);
    }
  }

  async function refreshStatus(code: string) {
    const status = await checkStatus(code);
    setStatuses((prev) => ({ ...prev, [code]: status }));
  }

  function removeLink(code: string) {
    setLinks((prev) => prev.filter((l) => l.code !== code));
  }

  return (
    <main className="container">
      <header>
        <h1>URL Shortener</h1>
        <p className="subtitle">
          API: <code>{API_BASE}</code>
        </p>
      </header>

      <form className="card" onSubmit={handleSubmit}>
        <label>
          Long URL
          <input
            type="url"
            required
            placeholder="https://example.com/very/long/path"
            value={longUrl}
            onChange={(e) => setLongUrl(e.target.value)}
          />
        </label>
        <div className="row">
          <label>
            Custom alias <span className="optional">(optional)</span>
            <input
              type="text"
              placeholder="my-custom-alias"
              value={alias}
              onChange={(e) => setAlias(e.target.value)}
            />
          </label>
          <label>
            Expires at <span className="optional">(optional)</span>
            <input
              type="datetime-local"
              value={expiresAt}
              onChange={(e) => setExpiresAt(e.target.value)}
            />
          </label>
        </div>
        <button type="submit" disabled={submitting}>
          {submitting ? "Shortening…" : "Shorten"}
        </button>
        {error && <p className="error">{error}</p>}
      </form>

      <section>
        <h2>Your links</h2>
        {links.length === 0 ? (
          <p className="empty">No links yet. Create one above.</p>
        ) : (
          <ul className="links">
            {links.map((link) => (
              <li key={link.code} className="card link">
                <div className="link-main">
                  <a href={link.short_url} target="_blank" rel="noreferrer">
                    {link.short_url}
                  </a>
                  {statuses[link.code] && (
                    <span className={`badge badge-${statuses[link.code]}`}>
                      {statuses[link.code]}
                    </span>
                  )}
                </div>
                <div className="link-long" title={link.long_url}>
                  → {link.long_url}
                </div>
                {link.expires_at && (
                  <div className="link-meta">
                    expires {new Date(link.expires_at).toLocaleString()}
                  </div>
                )}
                <div className="actions">
                  <button onClick={() => navigator.clipboard.writeText(link.short_url)}>
                    Copy
                  </button>
                  <button onClick={() => window.open(link.short_url, "_blank")}>
                    Test redirect
                  </button>
                  <button onClick={() => refreshStatus(link.code)}>Check status</button>
                  <button className="danger" onClick={() => removeLink(link.code)}>
                    Remove
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </main>
  );
}
