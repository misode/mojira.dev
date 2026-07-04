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
  document.querySelector('.image-overlay-arrow.left').onclick = prevAttachment
  document.querySelector('.image-overlay-arrow.right').onclick = nextAttachment
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
  const attachmentType = el?.dataset["filetype"]
  if (!overlay || !el || el.dataset["filetype"] === "unknown") return false
  
  const querySelectorElement = attachmentType === "image" ? "img" : "video"
  const attachmentSrc = el.querySelector(querySelectorElement).src
  const attachmentAlt = el.getAttribute('data-attachment-info')
  
  overlay.querySelector("img").src = querySelectorElement === "img" ? attachmentSrc : ""
  overlay.querySelector("img").alt = querySelectorElement === "img" ? attachmentAlt : ""
  overlay.querySelector("video").src = querySelectorElement === "video" ? attachmentSrc : ""
  overlay.querySelector("video").alt = querySelectorElement === "video" ? attachmentAlt : ""
  overlay.querySelector("video").controls = querySelectorElement === "video"
  overlay.querySelector('.image-overlay-info').textContent = el.getAttribute('data-attachment-info')

  const id = el.dataset["attachmentIdx"]
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

const attachmentElements = [...document.querySelectorAll("div.attachments > a.attachment")].filter(element => element.dataset["filetype"] !== "unknown");
attachmentElements.map((element, idx) => element.dataset["attachmentIdx"] = idx)

function prevAttachment() {
  let nextId = (parseInt(overlay?.dataset["currentId"]) - 1)
  if (nextId == -1) nextId += attachmentElements.length
  const attachment = attachmentElements[nextId]
  showAttachment(attachment)
}

function nextAttachment() {
  const nextId = (parseInt(overlay?.dataset["currentId"]) + 1) % attachmentElements.length
  const attachment = attachmentElements[nextId]
  showAttachment(attachment)
}
