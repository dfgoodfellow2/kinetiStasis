/**
 * Lightweight, dependency-free draft persistence for offline form entry.
 *
 * Drafts are stored as JSON in localStorage under a caller-provided key.
 * saveDraft is debounced (~300ms) so typing does not hammer storage; only the
 * final form object is written. loadDraft always returns a plain object or
 * null. clearDraft also cancels any pending write for that key, so a discard
 * or a successful submit cannot be undone by a timer that fires afterwards.
 *
 * No image blobs or binary payloads should be passed in — callers should
 * snapshot the text/number fields of their form only.
 */

const timers = new Map()

function getStorage() {
  try {
    if (typeof localStorage !== 'undefined') return localStorage
  } catch {
    // localStorage can throw in private modes / disabled-storage contexts.
  }
  return null
}

/**
 * Persist a form object after a short debounce.
 * @param {string} key
 * @param {object|null} obj
 * @param {number} [delay=300]
 */
export function saveDraft(key, obj, delay = 300) {
  if (!key) return
  const store = getStorage()
  if (!store) return

  const pending = timers.get(key)
  if (pending) clearTimeout(pending)

  // Serialize immediately so reactive proxies are read while the component is
  // still alive, and so later mutations cannot change what gets written.
  let json
  try {
    json = JSON.stringify(obj)
  } catch {
    return
  }
  if (json === undefined) return

  const timer = setTimeout(() => {
    timers.delete(key)
    try {
      if (json === 'null') store.removeItem(key)
      else store.setItem(key, json)
    } catch {
      // Quota exceeded / storage disabled — drafts are best-effort.
    }
  }, delay)

  timers.set(key, timer)
}

/**
 * Read and parse a draft. Returns null when absent or malformed.
 * @param {string} key
 * @returns {object|null}
 */
export function loadDraft(key) {
  if (!key) return null
  const store = getStorage()
  if (!store) return null
  try {
    const raw = store.getItem(key)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? parsed : null
  } catch {
    return null
  }
}

/**
 * Remove a draft and cancel any pending debounced write for it.
 * @param {string} key
 */
export function clearDraft(key) {
  if (!key) return
  const pending = timers.get(key)
  if (pending) {
    clearTimeout(pending)
    timers.delete(key)
  }
  const store = getStorage()
  if (!store) return
  try {
    store.removeItem(key)
  } catch {
    // Ignore storage errors — the in-memory timer is already cancelled.
  }
}
