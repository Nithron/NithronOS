export default function HelpProxy() {
  const curl = `curl -i http://YOUR_HOST/api/v1/setup/state\n# Expect: HTTP/1.1 200 and Content-Type: application/json`
  const caddy = `# API first\nhandle_path /api/* {\n  reverse_proxy localhost:8080\n}\n\n# SPA fallback\nhandle {\n  root * /srv/web\n  file_server\n  try_files {path} /index.html\n}`
  return (
    <div className="max-w-3xl mx-auto space-y-4">
      <h1 className="screen-title">Troubleshooting: Proxy for /api/*</h1>
      <p className="text-sm text-muted-foreground">
        Symptom: The frontend shows "Backend unreachable or proxy misconfigured" and requests to /api/* return HTML instead of JSON.
      </p>
      <h2 className="text-lg font-medium">Quick checks</h2>
      <pre className="bg-card rounded p-3 text-xs overflow-auto"><code>{curl}</code></pre>
      <h2 className="text-lg font-medium">Caddy minimal config</h2>
      <p className="text-sm">Ensure API routes are matched before the SPA fallback:</p>
      <pre className="bg-card rounded p-3 text-xs overflow-auto"><code>{caddy}</code></pre>
      <p className="text-sm text-muted-foreground">See full docs in Admin → Config.</p>
    </div>
  )
}


