const isWeb = import.meta.env.VITE_WEB === 'true'

function apiOrigin() {
  if (import.meta.env.VITE_API_BASE) {
    return import.meta.env.VITE_API_BASE
  }
  return ''
}

export function authFetch(path, options = {}) {
  return fetch(`${apiOrigin()}${path}`, {
    credentials: 'include',
    ...options,
    headers: {
      ...(options.body && !(options.body instanceof FormData) ? { 'Content-Type': 'application/json' } : {}),
      ...(options.headers || {}),
    },
  })
}

export async function fetchAuthStatus() {
  const res = await authFetch('/api/auth/status')
  if (!res.ok) {
    return { allowRegister: false, hasUsers: true }
  }
  return res.json()
}

export async function fetchMe() {
  const res = await authFetch('/api/auth/me')
  if (res.status === 401) {
    return null
  }
  if (!res.ok) {
    throw new Error(await res.text())
  }
  const body = await res.json()
  return body.user || null
}

async function readError(res) {
  try {
    const body = await res.json()
    if (body && body.error) {
      return body.error
    }
  } catch {
    /* ignore */
  }
  return res.statusText || '请求失败'
}

export async function login(username, password) {
  const res = await authFetch('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
  if (!res.ok) {
    throw new Error(await readError(res))
  }
  const body = await res.json()
  return body.user
}

export async function register(username, password) {
  const res = await authFetch('/api/auth/register', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
  if (!res.ok) {
    throw new Error(await readError(res))
  }
  const body = await res.json()
  return body.user
}

export async function logout() {
  await authFetch('/api/auth/logout', { method: 'POST' })
}

export async function listUsers() {
  const res = await authFetch('/api/auth/users')
  if (!res.ok) {
    throw new Error(await readError(res))
  }
  const body = await res.json()
  return body.users || []
}

export async function createUser(username, password, isAdmin) {
  const res = await authFetch('/api/auth/users', {
    method: 'POST',
    body: JSON.stringify({ username, password, isAdmin: !!isAdmin }),
  })
  if (!res.ok) {
    throw new Error(await readError(res))
  }
  const body = await res.json()
  return body.user
}

export async function setUserDisabled(id, disabled) {
  const res = await authFetch(`/api/auth/users/${id}/disabled`, {
    method: 'POST',
    body: JSON.stringify({ disabled: !!disabled }),
  })
  if (!res.ok) {
    throw new Error(await readError(res))
  }
}

export function isWebMode() {
  return isWeb
}
