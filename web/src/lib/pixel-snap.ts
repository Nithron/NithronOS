/**
 * Pixel text is sharp only on whole device pixels. Layout can put it on a
 * fraction (a grid column, a flex row, a centred box), where the browser
 * smooths every edge. After layout, on resize, on font load and when the
 * page changes, each .pixel-display element is moved by a relative offset
 * of less than a pixel so its box starts on a whole device pixel.
 */
const SELECTOR = '.pixel-display'

function snap(el: HTMLElement): void {
  el.style.position = 'relative'
  el.style.left = '0px'
  el.style.top = '0px'
  const dpr = window.devicePixelRatio || 1
  const r = el.getBoundingClientRect()
  const dx = Math.round(r.left * dpr) / dpr - r.left
  const dy = Math.round(r.top * dpr) / dpr - r.top
  if (dx !== 0) el.style.left = `${dx}px`
  if (dy !== 0) el.style.top = `${dy}px`
}

export function snapPixelText(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>(SELECTOR).forEach(snap)
}

/** Keeps every pixel text on whole pixels; returns a function that stops it. */
export function startPixelSnap(): () => void {
  let frame = 0
  const schedule = () => {
    cancelAnimationFrame(frame)
    frame = requestAnimationFrame(() => snapPixelText())
  }
  const observer = new MutationObserver((records) => {
    // Our own style changes are attribute changes on pixel text: ignore them.
    if (records.every((r) => r.type === 'attributes' && (r.target as Element).matches?.(SELECTOR))) return
    schedule()
  })
  observer.observe(document.body, { childList: true, subtree: true, characterData: true, attributes: true, attributeFilter: ['class', 'style'] })
  window.addEventListener('resize', schedule)
  document.fonts?.ready.then(schedule).catch(() => {})
  document.fonts?.addEventListener?.('loadingdone', schedule)
  schedule()
  return () => {
    observer.disconnect()
    window.removeEventListener('resize', schedule)
    document.fonts?.removeEventListener?.('loadingdone', schedule)
    cancelAnimationFrame(frame)
  }
}
