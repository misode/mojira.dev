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
    el.onclick = (e) => {
      if (e.ctrlKey || e.shiftKey) {
        return // Don't overwrite opening attachments in new tab
      }
      let success = showAttachment(el)
      if (success) {
        e.preventDefault()
      }
    }
  })

  document.querySelectorAll('.file-overlay-backdrop').forEach((el) => {
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

const overlay = document.getElementById('file-overlay')

if (overlay) {
  document.querySelector('.file-overlay-arrow.left').onclick = prevAttachment
  document.querySelector('.file-overlay-arrow.right').onclick = nextAttachment
  document.addEventListener('keydown', (e) => {
    if (overlay.hasAttribute('data-current-id')) {
      if (e.key === 'Escape') closeOverlay()
      if (e.key === 'ArrowLeft') prevAttachment()
      if (e.key === 'ArrowRight') nextAttachment()
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
  if (!overlay || !el) {
    return false
  }
  const fileType = el.getAttribute('data-attachment-type')
  if (fileType === 'unknown') {
    return false
  }
  overlay.querySelector('img').src = ''
  overlay.querySelector('video').src = ''
  if (fileType === 'image') {
    overlay.querySelector('img').src = el.getAttribute('href')
    overlay.querySelector('img').alt = el.getAttribute('data-attachment-info')
  } else if (fileType === 'video') {
    overlay.querySelector('video').src = el.getAttribute('href')
    overlay.querySelector('video').alt = el.getAttribute('data-attachment-info')
    overlay.querySelector('video').controls = true
  } else {
    return false
  }
  overlay.querySelector('.file-overlay-info').textContent = el.getAttribute('data-attachment-info')
  const id = el.getAttribute('data-attachment')
  overlay.setAttribute('data-current-id', id)
  const url = new URL(window.location)
  url.searchParams.set('attachment', id)
  window.history.replaceState({}, '', url)
  return true
}

function closeOverlay() {
  if (overlay) {
    overlay.querySelector('video').src = ''
    overlay.querySelector('img').src = ''
    overlay.removeAttribute('data-current-id')
  }
  const url = new URL(window.location)
  url.searchParams.delete('attachment')
  window.history.replaceState({}, '', url)
}

function prevAttachment() {
  const currentEl = document.querySelector(`[data-attachment="${overlay.getAttribute('data-current-id')}"]`)
  let el = currentEl?.previousElementSibling
  while (el && !el.matches(':not([data-attachment-type="unknown"])')) {
    el = el.previousElementSibling
  }
  showAttachment(el)
}

function nextAttachment() {
  const currentEl = document.querySelector(`[data-attachment="${overlay.getAttribute('data-current-id')}"]`)
  let el = currentEl?.nextElementSibling
  while (el && !el.matches(':not([data-attachment-type="unknown"])')) {
    el = el.nextElementSibling
  }
  showAttachment(el)
}
