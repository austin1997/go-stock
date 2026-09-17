import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const appJs = path.resolve(__dirname, '../wailsjs/go/main/App.js')
const outFile = path.resolve(__dirname, '../src/platform/app-web-generated.js')

const SPECIAL = new Set([
  'ImportSkillPackage',
  'ImportTradingRecordsFromExcel',
  'PickKBFilePath',
  'PickKBFilePaths',
  'OpenURL',
  'RestartAsAdmin',
  'HideToTray',
  'ShowFromTray',
  'QuitApp',
])

const src = fs.readFileSync(appJs, 'utf8')
const names = [...src.matchAll(/^export function (\w+)\(/gm)].map((m) => m[1])

const lines = [
  '// Generated from frontend/wailsjs/go/main/App.js — do not edit.',
  "import { rpc } from './rpc.js'",
  '',
]

for (const name of names) {
  if (SPECIAL.has(name)) {
    continue
  }
  const fn = src.match(new RegExp(`export function ${name}\\(([^)]*)\\)`))
  const args = fn && fn[1] ? fn[1].split(',').map((s) => s.trim()).filter(Boolean) : []
  const argList = args.join(', ')
  const callArgs = args.length ? ', ' + args.join(', ') : ''
  lines.push(`export function ${name}(${argList}) {`)
  lines.push(`  return rpc('${name}'${callArgs});`)
  lines.push(`}`)
  lines.push('')
}

fs.mkdirSync(path.dirname(outFile), { recursive: true })
fs.writeFileSync(outFile, lines.join('\n'))
console.log(`generated ${outFile} (${names.length - SPECIAL.size} methods)`)
