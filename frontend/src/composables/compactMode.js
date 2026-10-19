const STORAGE_KEY = 'plist-compact-mode'
const CLASS_NAME = 'compact-mode'

function readStored() {
  try {
    return localStorage.getItem(STORAGE_KEY) === '1'
  } catch {
    return false
  }
}

function writeStored(enabled) {
  try {
    localStorage.setItem(STORAGE_KEY, enabled ? '1' : '0')
  } catch {
    // Storage can be unavailable in private browsing.
  }
}

export function applyCompactMode(enabled) {
  document.documentElement.classList.toggle(CLASS_NAME, !!enabled)
}

/** Apply stored preference before Vue mounts to avoid a layout flash. */
export function initCompactMode() {
  applyCompactMode(readStored())
}

export function isCompactMode() {
  return document.documentElement.classList.contains(CLASS_NAME)
}

export function setCompactMode(enabled) {
  const on = !!enabled
  applyCompactMode(on)
  writeStored(on)
  return on
}

export function toggleCompactMode() {
  return setCompactMode(!isCompactMode())
}
