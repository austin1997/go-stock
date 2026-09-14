function apiOrigin() {
  if (import.meta.env.VITE_API_BASE) {
    return import.meta.env.VITE_API_BASE
  }
  return ''
}

export async function authStatus() {
  const res = await fetch(`${apiOrigin()}/api/auth/status`, {
    credentials: 'include',
  })
  if (!res.ok) {
    return { required: true, authenticated: false }
  }
  return res.json()
}

async function login(token) {
  const res = await fetch(`${apiOrigin()}/api/auth/login`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token }),
  })
  const body = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error(body.error || `登录失败 (${res.status})`)
  }
}

let inflight = null

export function ensureAuthenticated({ force = false } = {}) {
  if (inflight) {
    return inflight
  }
  inflight = (async () => {
    try {
      if (!force) {
        const st = await authStatus()
        if (st.authenticated) {
          return
        }
      }
      await promptLogin()
    } finally {
      inflight = null
    }
  })()
  return inflight
}

function promptLogin() {
  return new Promise((resolve) => {
    const existing = document.getElementById('go-stock-web-login')
    if (existing) {
      existing.remove()
    }
    const wrap = document.createElement('div')
    wrap.id = 'go-stock-web-login'
    wrap.innerHTML = `
      <style>
        #go-stock-web-login { position:fixed; inset:0; z-index:99999; display:flex; align-items:center; justify-content:center;
          background:#0f172a; font-family: system-ui, -apple-system, "Segoe UI", sans-serif; color:#e2e8f0; }
        #go-stock-web-login .card { width:min(420px, calc(100vw - 32px)); background:#1e293b; border:1px solid #334155;
          border-radius:12px; padding:28px 24px; box-shadow:0 20px 50px rgba(0,0,0,.35); }
        #go-stock-web-login h1 { margin:0 0 8px; font-size:20px; font-weight:650; }
        #go-stock-web-login p { margin:0 0 18px; font-size:13px; line-height:1.6; color:#94a3b8; }
        #go-stock-web-login input { width:100%; box-sizing:border-box; padding:10px 12px; border-radius:8px;
          border:1px solid #475569; background:#0f172a; color:#e2e8f0; font-size:14px; }
        #go-stock-web-login button { margin-top:14px; width:100%; padding:10px 12px; border:0; border-radius:8px;
          background:#2563eb; color:#fff; font-size:14px; cursor:pointer; }
        #go-stock-web-login button:disabled { opacity:.6; cursor:default; }
        #go-stock-web-login .err { min-height:18px; margin:10px 0 0; font-size:12px; color:#f87171; }
      </style>
      <form class="card">
        <h1>go-stock 网页版</h1>
        <p>请输入访问口令（环境变量 WEB_AUTH_TOKEN，或文件 data/.web_auth_token）。</p>
        <input type="password" name="token" autocomplete="current-password" placeholder="访问口令" />
        <button type="submit">登录</button>
        <div class="err"></div>
      </form>
    `
    document.body.appendChild(wrap)
    const form = wrap.querySelector('form')
    const input = wrap.querySelector('input')
    const errEl = wrap.querySelector('.err')
    const btn = wrap.querySelector('button')
    input.focus()
    form.addEventListener('submit', async (ev) => {
      ev.preventDefault()
      const token = input.value.trim()
      if (!token) {
        errEl.textContent = '请输入访问口令'
        return
      }
      btn.disabled = true
      errEl.textContent = ''
      try {
        await login(token)
        wrap.remove()
        window.dispatchEvent(new Event('go-stock-web-authenticated'))
        resolve()
      } catch (err) {
        errEl.textContent = err.message || '登录失败'
        btn.disabled = false
      }
    })
  })
}
