// Pins the theme an administrator chose (Settings, Theme) before the page
// is drawn, so it never flashes the other one. Without a stored choice the
// tokens follow the system. lib/theme.ts keeps the stored value up to date.
;(function () {
  try {
    var t = localStorage.getItem('nos.theme')
    if (t === 'light' || t === 'dark') document.documentElement.setAttribute('data-theme', t)
  } catch (e) {
    // Storage unavailable: follow the system.
  }
})()
