import { getClientId } from './rpc.js'

const listeners = new Map()
let socket = null
let reconnectTimer = null

function dispatch(name, data) {
  const set = listeners.get(name)
  if (!set) {
    return
  }
  for (const cb of Array.from(set)) {
    try {
      cb(data)
    } catch (err) {
      console.error(err)
    }
  }
}

function wsUrl() {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const base = import.meta.env.VITE_API_BASE
  if (base) {
    const u = new URL(base, location.href)
    const wsProto = u.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${wsProto}//${u.host}/api/ws?clientId=${encodeURIComponent(getClientId())}`
  }
  return `${proto}//${location.host}/api/ws?clientId=${encodeURIComponent(getClientId())}`
}

function connect() {
  if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
    return
  }
  try {
    socket = new WebSocket(wsUrl())
  } catch (err) {
    scheduleReconnect()
    return
  }
  socket.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data)
      if (msg && msg.name) {
        if (msg.name === 'openURL' && typeof msg.data === 'string') {
          window.open(msg.data, '_blank')
        }
        dispatch(msg.name, msg.data)
      }
    } catch (err) {
      console.error(err)
    }
  }
  socket.onclose = () => scheduleReconnect()
  socket.onerror = () => {
    try {
      socket.close()
    } catch {
      /* ignore */
    }
  }
}

function scheduleReconnect() {
  if (reconnectTimer) {
    return
  }
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    connect()
  }, 2000)
}

connect()

function removeCallback(eventName, callback) {
  const set = listeners.get(eventName)
  if (!set) {
    return
  }
  set.delete(callback)
  if (set.size === 0) {
    listeners.delete(eventName)
  }
}

export function EventsOnMultiple(eventName, callback, maxCallbacks) {
  return EventsOn(eventName, callback)
}

export function EventsOn(eventName, callback) {
  if (!listeners.has(eventName)) {
    listeners.set(eventName, new Set())
  }
  listeners.get(eventName).add(callback)
  return () => removeCallback(eventName, callback)
}

export function EventsOff(eventName, ...additionalEventNames) {
  const extraCallbacks = additionalEventNames.filter((arg) => typeof arg === 'function')
  const extraNames = additionalEventNames.filter((arg) => typeof arg === 'string')
  if (extraCallbacks.length > 0) {
    for (const cb of extraCallbacks) {
      removeCallback(eventName, cb)
    }
  } else {
    listeners.delete(eventName)
  }
  for (const name of extraNames) {
    listeners.delete(name)
  }
}

export function EventsOffAll() {
  listeners.clear()
}

export function EventsOnce(eventName, callback) {
  const wrap = (data) => {
    removeCallback(eventName, wrap)
    callback(data)
  }
  return EventsOn(eventName, wrap)
}

export function EventsEmit(eventName, ...data) {
  const payload = data.length <= 1 ? data[0] : data
  dispatch(eventName, payload)
  if (eventName === 'frontendError' && socket && socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify({ type: 'emit', name: eventName, data: payload }))
  }
}

export function LogPrint(message) { console.log(message) }
export function LogTrace(message) { console.debug(message) }
export function LogDebug(message) { console.debug(message) }
export function LogInfo(message) { console.info(message) }
export function LogWarning(message) { console.warn(message) }
export function LogError(message) { console.error(message) }
export function LogFatal(message) { console.error(message) }

export function WindowReload() { location.reload() }
export function WindowReloadApp() { location.reload() }
export function WindowSetAlwaysOnTop() {}
export function WindowSetSystemDefaultTheme() {}
export function WindowSetLightTheme() {}
export function WindowSetDarkTheme() {}
export function WindowCenter() {}
export function WindowSetTitle(title) {
  if (title) {
    document.title = title
  }
}
export function WindowFullscreen() {
  document.documentElement.requestFullscreen?.()
}
export function WindowUnfullscreen() {
  document.exitFullscreen?.()
}
export function WindowIsFullscreen() {
  return Promise.resolve(!!document.fullscreenElement)
}
export function WindowGetSize() {
  return Promise.resolve({ w: window.innerWidth, h: window.innerHeight })
}
export function WindowSetSize() {}
export function WindowSetMaxSize() {}
export function WindowSetMinSize() {}
export function WindowSetPosition() {}
export function WindowGetPosition() {
  return Promise.resolve({ x: 0, y: 0 })
}
export function WindowHide() {}
export function WindowShow() {}
export function WindowMaximise() {}
export function WindowToggleMaximise() {}
export function WindowUnmaximise() {}
export function WindowIsMaximised() { return Promise.resolve(false) }
export function WindowMinimise() {}
export function WindowUnminimise() {}
export function WindowIsMinimised() { return Promise.resolve(false) }
export function WindowIsNormal() { return Promise.resolve(true) }
export function WindowSetBackgroundColour() {}
export function ScreenGetAll() {
  return Promise.resolve([{ isCurrent: true, isPrimary: true, width: window.screen.width, height: window.screen.height }])
}

export function BrowserOpenURL(url) {
  if (url) {
    window.open(url, '_blank')
  }
}

export function Environment() {
  return Promise.resolve({
    buildType: 'web',
    platform: 'web',
    arch: 'browser',
  })
}

export function Quit() {}
export function Hide() {}
export function Show() {}
export function ClipboardGetText() {
  return navigator.clipboard?.readText?.() ?? Promise.resolve('')
}
export function ClipboardSetText(text) {
  return navigator.clipboard?.writeText?.(text) ?? Promise.resolve()
}

export function OnFileDrop() {}
export function OnFileDropOff() {}
export function CanResolveFilePaths() { return false }
export function ResolveFilePaths() { return Promise.resolve([]) }

