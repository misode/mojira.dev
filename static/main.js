function afterSwap() {
  document.querySelectorAll('time[datetime]').forEach((el) => {
    const d = new Date(el.getAttribute('datetime'))
    if (!isNaN(d)) {
      el.textContent = d.toLocaleString()
    }
  })

  document.querySelectorAll('[data-copy]').forEach((el) => {
    let copiedTimer
    el.onclick = () => {
      const value = el.getAttribute('data-copy')
      navigator.clipboard.writeText(value)
      el.classList.add('copied')
      clearTimeout(copiedTimer)
      copiedTimer = setTimeout(() => el.classList.remove('copied'), 3000)
    }
  })

  document.querySelectorAll('[data-attachment]').forEach((el) => {
    if (!el.querySelector('img')) return
    el.onclick = (e) => {
      let success = showAttachment(el)
      if (success) {
        e.preventDefault()
      }
    }
  })

  document.querySelectorAll('.image-overlay-backdrop').forEach((el) => {
    el.onclick = closeOverlay
  })

  document.querySelectorAll('[data-expand]').forEach((el) => {
    el.onclick = () => {
      const contents = document.getElementById(el.getAttribute('data-expand'))
      if (contents) {
        el.outerHTML = contents.innerHTML
      }
    }
  })

  expandCommentsIfNeeded()
}

function onHashChange() {
  expandCommentsIfNeeded()
  const hash = window.location.hash
  if (hash) {
    document.querySelector(hash)?.scrollIntoView({ block: 'start' })
  }
}

function expandCommentsIfNeeded() {
  const hash = window.location.hash
  if (hash && hash.startsWith('#comment-')) {
    const el = document.querySelector('[data-expand=hidden-comments]')
    if (el && el.querySelector(hash)) {
      const contents = document.getElementById(el.getAttribute('data-expand'))
      if (contents) {
        el.outerHTML = contents.innerHTML
      }
    }
  }
  document.querySelectorAll('.comment-highlighted').forEach((el) => el.classList.remove('comment-highlighted'))
  if (hash) {
    document.querySelector(hash)?.parentElement?.classList.add('comment-highlighted')
  }
}

afterSwap()

setTimeout(() => {
  onHashChange()
}, 500)

document.body.addEventListener('htmx:afterSwap', () => {
  afterSwap()
})

window.addEventListener('hashchange', () => {
  onHashChange()
})

const overlay = document.getElementById('image-overlay')

if (overlay) {
  document.querySelector('.image-overlay-arrow.left').onclick = prevImg
  document.querySelector('.image-overlay-arrow.right').onclick = nextImg
  document.addEventListener('keydown', (e) => {
    if (overlay.hasAttribute('data-current-id')) {
      if (e.key === 'Escape') closeOverlay()
      if (e.key === 'ArrowLeft') prevImg()
      if (e.key === 'ArrowRight') nextImg()
    }
  })
  const params = new URL(window.location).searchParams
  const attachment = params.get('attachment')
  if (attachment) {
    const el = document.querySelector(`[data-attachment="${attachment}"]`)
    showAttachment(el)
  }
}

function showAttachment(el) {
  if (!overlay || !el) return false
  overlay.querySelector('img').src = el.querySelector('img').src
  overlay.querySelector('.image-overlay-info').textContent = el.getAttribute('data-attachment-info')
  const id = el.getAttribute('data-attachment')
  overlay.setAttribute('data-current-id', id)
  const url = new URL(window.location)
  url.searchParams.set('attachment', id)
  window.history.replaceState({}, '', url)
  return true
}

function closeOverlay() {
  overlay?.removeAttribute('data-current-id')
  const url = new URL(window.location)
  url.searchParams.delete('attachment')
  window.history.replaceState({}, '', url)
}

function prevImg() {
  const current = overlay?.getAttribute('data-current-id')
  const a = document.querySelector(`[data-attachment="${current}"]`)?.previousElementSibling
  showAttachment(a)
}

function nextImg() {
  const current = overlay?.getAttribute('data-current-id')
  const a = document.querySelector(`[data-attachment="${current}"]`)?.nextElementSibling
  showAttachment(a)
}


initTheme()

function initTheme() {
  const stored = localStorage.getItem('theme')
  const system = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  const theme = stored === 'system' || !stored ? 'system' : stored

  applyTheme(theme)

  const actualTheme = theme === 'system' ? system : theme
  document.cookie = `theme=${actualTheme}; path=/; max-age=31536000; SameSite=Lax`
}

function applyTheme(theme) {
  const isDark = theme === 'dark' || (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.setAttribute('data-theme', isDark ? 'dark' : 'light')

  document.querySelectorAll('.theme-option').forEach(btn => {
    btn.classList.toggle('active', btn.getAttribute('data-value') === theme)
  })

  updateThemeIcon(theme)
}

function setTheme(theme) {
  localStorage.setItem('theme', theme)
  applyTheme(theme)

  const actualTheme = theme === 'system'
    ? (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
    : theme
  document.cookie = `theme=${actualTheme}; path=/; max-age=31536000; SameSite=Lax`

  const details = document.querySelector('.theme-dropdown')
  if (details) {
    details.removeAttribute('open')
  }
}

function updateThemeIcon(theme) {
  const option = document.querySelector(`.theme-option[data-value="${theme}"] svg`)
  const toggle = document.querySelector('.theme-toggle')
  if (option && toggle) {
    toggle.innerHTML = option.outerHTML
  }
}

document.addEventListener('click', (e) => {
  const details = document.querySelector('.theme-dropdown')
  if (details && details.hasAttribute('open') && !details.contains(e.target)) {
    details.removeAttribute('open')
  }
})

window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', e => {
  if (!localStorage.getItem('theme') || localStorage.getItem('theme') === 'system') {
    applyTheme('system')
  }
})

initTheme()

window.setTheme = setTheme
