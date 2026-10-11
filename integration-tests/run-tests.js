import { createServer } from 'node:net'
import { spawn } from 'node:child_process'
import { existsSync, mkdirSync, rmSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import waitOn from 'wait-on'

const __dirname = dirname(fileURLToPath(import.meta.url))
const repoRoot = join(__dirname, '..')
const testsDir = join(__dirname, 'tests')
const serviceBin = join(repoRoot, 'plist-wui')
const frontendDist = join(repoRoot, 'frontend', 'dist')
const testDb = join(repoRoot, 'tmp', 'repomanager-test.db')

async function getFreePort() {
  return new Promise((resolve, reject) => {
    const server = createServer()
    server.listen(0, '127.0.0.1', () => {
      const address = server.address()
      const port = typeof address === 'object' && address ? address.port : 0
      server.close((err) => (err ? reject(err) : resolve(port)))
    })
    server.on('error', reject)
  })
}

function resetTestDatabase() {
  mkdirSync(join(repoRoot, 'tmp'), { recursive: true })
  for (const suffix of ['', '-wal', '-shm']) {
    try {
      rmSync(testDb + suffix)
    } catch {
      // ignore
    }
  }
}

async function main() {
  if (!existsSync(serviceBin)) {
    throw new Error(`Service binary not found at ${serviceBin}. Run make build first.`)
  }
  if (!existsSync(join(frontendDist, 'index.html'))) {
    throw new Error(`Frontend build not found at ${frontendDist}. Run make frontend first.`)
  }

  resetTestDatabase()
  mkdirSync(join(repoRoot, 'tmp', 'httpauthshim-test'), { recursive: true })
  await new Promise((resolve, reject) => {
    const migrate = spawn('make', ['-C', join(repoRoot, 'database', 'sqlite')], {
      env: { ...process.env, DB_PATH: testDb },
      stdio: 'inherit',
    })
    migrate.on('exit', (code) => (code === 0 ? resolve() : reject(new Error(`migrate exit ${code}`))))
  })

  const testPort = process.env.REPOMANAGER_TEST_PORT
    ? Number(process.env.REPOMANAGER_TEST_PORT)
    : await getFreePort()

  const env = {
    ...process.env,
    PORT: String(testPort),
    REPOMANAGER_SECURE_COOKIES: 'false',
    REPOMANAGER_DEV_DISABLE_AUTH: 'true',
    PLIST_STATIC_DIR: frontendDist,
  }

  const child = spawn(serviceBin, ['-configdir', testsDir], {
    cwd: repoRoot,
    env,
    stdio: ['ignore', 'inherit', 'inherit'],
  })

  try {
    await waitOn({
      resources: [`http-get://127.0.0.1:${testPort}/`],
      timeout: 30000,
      interval: 250,
    })
    await new Promise((resolve, reject) => {
      const test = spawn('npx', ['mocha', 'tests/init.spec.js'], {
        cwd: __dirname,
        env: { ...env, REPOMANAGER_BASE_URL: `http://127.0.0.1:${testPort}` },
        stdio: 'inherit',
      })
      test.on('exit', (code) => (code === 0 ? resolve() : reject(new Error(`mocha exit ${code}`))))
    })
  } finally {
    child.kill('SIGTERM')
  }
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
