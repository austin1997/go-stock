import { rpc, pickFiles, uploadFile, fileToBase64 } from './rpc.js'

export async function ImportSkillPackage() {
  const files = await pickFiles({ accept: '.zip,application/zip', multiple: false })
  if (!files.length) {
    return '未选择文件'
  }
  const b64 = await fileToBase64(files[0])
  return rpc('ImportSkillFromBase64', b64)
}

export async function ImportTradingRecordsFromExcel() {
  const files = await pickFiles({ accept: '.xls,.xlsx,.txt,.csv', multiple: false })
  if (!files.length) {
    return null
  }
  const path = await uploadFile(files[0])
  return rpc('ImportTradingRecordsFromPath', path)
}

export async function PickKBFilePath() {
  const files = await pickFiles({ accept: '.txt,.md,text/plain,text/markdown', multiple: false })
  if (!files.length) {
    return ''
  }
  return uploadFile(files[0])
}

export async function PickKBFilePaths() {
  const files = await pickFiles({ accept: '.txt,.md,text/plain,text/markdown', multiple: true })
  if (!files.length) {
    return []
  }
  const paths = []
  for (const file of files) {
    paths.push(await uploadFile(file))
  }
  return paths
}

export function OpenURL(url) {
  if (url) {
    window.open(url, '_blank')
  }
}

export function RestartAsAdmin() {
  return Promise.resolve(null)
}

export function HideToTray() {}
export function ShowFromTray() {}
export function QuitApp() {}
