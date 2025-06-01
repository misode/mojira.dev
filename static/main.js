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
}

afterSwap()

document.body.addEventListener('htmx:afterSwap', () => {
  afterSwap()
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
  // Update URL with attachment param
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
