import { ensureAuthenticated } from './web-auth.js'

const clientId = (typeof crypto !== 'undefined' && crypto.randomUUID)
  ? crypto.randomUUID()
  : `web-${Date.now()}-${Math.random().toString(16).slice(2)}`

export function getClientId() {
  return clientId
}

function apiOrigin() {
  if (import.meta.env.VITE_API_BASE) {
    return import.meta.env.VITE_API_BASE
  }
  return ''
}

function requestHeaders(extra = {}) {
  return {
    'X-Client-Id': clientId,
    ...extra,
  }
}

export function consumeDownloadToken(result) {
  if (typeof result !== 'string') {
    return result
  }
  const marker = 'WEB_DOWNLOAD:'
  const idx = result.indexOf(marker)
  if (idx === -1) {
    return result
  }
  const rest = result.slice(idx + marker.length)
  const sep = rest.indexOf(':')
  const id = sep === -1 ? rest : rest.slice(0, sep)
  const filename = sep === -1 ? 'download' : rest.slice(sep + 1)
  const a = document.createElement('a')
  a.href = `${apiOrigin()}/api/download/${id}`
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  return '已下载：' + filename
}

async function rpcOnce(method, args) {
  const res = await fetch(`${apiOrigin()}/api/rpc`, {
    method: 'POST',
    credentials: 'include',
    headers: requestHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify({ method, args }),
  })
  if (res.status === 401) {
    const err = new Error('unauthorized')
    err.status = 401
    throw err
  }
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || `RPC HTTP ${res.status}`)
  }
  const body = await res.json()
  if (body && body.error) {
    throw new Error(body.error)
  }
  return consumeDownloadToken(body ? body.result : undefined)
}

export async function rpc(method, ...args) {
  try {
    return await rpcOnce(method, args)
  } catch (err) {
    if (err && err.status === 401) {
      await ensureAuthenticated({ force: true })
      return rpcOnce(method, args)
    }
    throw err
  }
}

export function pickFiles({ accept = '', multiple = false } = {}) {
  return new Promise((resolve) => {
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = accept
    input.multiple = multiple
    let settled = false
    const finish = (files) => {
      if (settled) {
        return
      }
      settled = true
      resolve(files)
    }
    input.addEventListener('change', () => finish(Array.from(input.files || [])))
    input.addEventListener('cancel', () => finish([]))
    input.click()
  })
}

export async function uploadFile(file) {
  const form = new FormData()
  form.append('file', file)
  const res = await fetch(`${apiOrigin()}/api/upload`, {
    method: 'POST',
    credentials: 'include',
    headers: requestHeaders(),
    body: form,
  })
  if (res.status === 401) {
    await ensureAuthenticated({ force: true })
    return uploadFile(file)
  }
  const data = await res.json()
  if (!res.ok || data.error) {
    throw new Error(data.error || '上传失败')
  }
  return data.path
}

export function fileToBase64(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      const result = String(reader.result || '')
      const comma = result.indexOf(',')
      resolve(comma >= 0 ? result.slice(comma + 1) : result)
    }
    reader.onerror = reject
    reader.readAsDataURL(file)
  })
}
