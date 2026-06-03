/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
// check-i18n-coverage.mjs
//
// CI gate: every t('...') key used in source code MUST be present in
// web/default/src/i18n/locales/en.json (the flat-key source of truth per
// CLAUDE.md). Missing keys cause i18next to render the raw English key on
// every locale, including zh — exactly the "中文页面也显示英文" bug fixed
// in audit follow-up T035 (see 010-sensitive-filter-p2-4/tasks.md).
//
// Exit code 0 = all keys present, 1 = some keys missing, 2 = runtime error.

import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const HERE = path.dirname(fileURLToPath(import.meta.url))
const PKG_ROOT = path.resolve(HERE, '..')
const SRC_ROOT = path.join(PKG_ROOT, 'src')
const LOCALES_DIR = path.join(SRC_ROOT, 'i18n/locales')
const BASE_LOCALE_FILE = path.join(LOCALES_DIR, 'en.json')

const SOURCE_DIRS = ['features', 'components', 'pages', 'hooks', 'context', 'helpers', 'routes', 'i18n']
const SOURCE_EXTS = new Set(['.ts', '.tsx', '.js', '.jsx'])
const IGNORE_DIRS = new Set(['node_modules', 'dist', 'build', '.rspeedy', '.turbo'])

// t('foo') / t("foo") with simple content. The codebase uses flat keys where
// the key IS the English source string, so we only need to catch string literals.
const T_CALL_RE = /\bt\(\s*(['"])((?:\\.|(?!\1).)*?)\1\s*[,)]/g

async function listSourceFiles(dir) {
  const out = []
  async function walk(d) {
    let entries
    try {
      entries = await fs.readdir(d, { withFileTypes: true })
    } catch {
      return
    }
    for (const e of entries) {
      if (IGNORE_DIRS.has(e.name)) continue
      const p = path.join(d, e.name)
      if (e.isDirectory()) {
        await walk(p)
      } else if (SOURCE_EXTS.has(path.extname(e.name))) {
        out.push(p)
      }
    }
  }
  await walk(dir)
  return out
}

async function collectSourceKeys() {
  const files = []
  for (const d of SOURCE_DIRS) {
    const abs = path.join(SRC_ROOT, d)
    files.push(...(await listSourceFiles(abs)))
  }
  const keys = new Set()
  for (const f of files) {
    const text = await fs.readFile(f, 'utf8')
    let m
    T_CALL_RE.lastIndex = 0
    while ((m = T_CALL_RE.exec(text)) !== null) {
      keys.add(m[2])
    }
  }
  return keys
}

function loadBaseKeysSync() {
  const { readFileSync } = require('node:fs')
  const raw = JSON.parse(readFileSync(BASE_LOCALE_FILE, 'utf8'))
  return new Set(Object.keys(raw.translation ?? raw))
}

async function main() {
  // Use sync fs to keep the script self-contained (no CJS/ESM bridge drama).
  const { readFileSync } = await import('node:fs')
  const baseRaw = JSON.parse(readFileSync(BASE_LOCALE_FILE, 'utf8'))
  const baseKeys = new Set(Object.keys(baseRaw.translation ?? baseRaw))

  const usedKeys = await collectSourceKeys()
  const missing = [...usedKeys].filter((k) => !baseKeys.has(k)).sort()

  if (missing.length === 0) {
    console.log(
      `i18n coverage OK — ${usedKeys.size} t() keys used, all present in en.json (${baseKeys.size} total).`,
    )
    process.exit(0)
  }

  console.error(
    `i18n coverage FAILED — ${missing.length} t() key(s) missing in en.json:`,
  )
  for (const k of missing) console.error(`  - ${k}`)
  console.error(
    `\nFix: add the key (and translations) under "translation" in ` +
      `src/i18n/locales/{en,zh,fr,ja,ru,vi}.json. ` +
      `See spec 010-sensitive-filter-p2-4/tasks.md T035.`,
  )
  process.exit(1)
}

main().catch((err) => {
  console.error(err)
  process.exit(2)
})
