/** @typedef {CustomEvent<{elt?: Element}> & {target: Element}} HtmxEvent */

/**
 * @typedef {Object} HtmxExtension
 * @property {(api: object) => void} [init]
 * @property {(name: string, event: HtmxEvent) => boolean} [onEvent]
 */

/**
 * @param {string} name
 * @param {() => HtmxExtension} builder
 */
function registerHtmxExtension(name, builder) {
  htmx.defineExtension(name, builder())
}

// Formats timestamps in the user locale
registerHtmxExtension('localized-time', () => {
  /**
   * @param {Element} root
   */
  function formatTimes(root) {
    const elements = root.matches?.('time[datetime]') ? [root] : []
    elements.push(...root.querySelectorAll('time[datetime]'))
    elements.forEach((el) => {
      const date = new Date(el.getAttribute('datetime'))
      if (!isNaN(date)) {
        el.textContent = date.toLocaleString()
      }
    })
  }

  return {
    init() {
      formatTimes(document)
    },

    onEvent(name, event) {
      if (name === 'htmx:afterSwap') {
        formatTimes(event.target)
      }
      return true
    }
  }
})

// Copies values from [data-copy] elements
registerHtmxExtension('copy-to-clipboard', () => {
  const copiedTimers = new WeakMap()

  return {
    init() {
      document.addEventListener('click', (event) => {
        const element = event.target.closest?.('[data-copy]')
        if (!element) {
          return
        }
        navigator.clipboard.writeText(element.getAttribute('data-copy'))
        element.classList.add('copied')
        clearTimeout(copiedTimers.get(element))
        copiedTimers.set(element, setTimeout(() => element.classList.remove('copied'), 3000))
      })
    }
  }
})

// Expandable content (comments, linked issues, affects versions)
registerHtmxExtension('content-expand', () => ({
  init() {
    document.addEventListener('click', (event) => {
      const element = event.target.closest?.('[data-expand]')
      if (!element) {
        return
      }
      const contents = document.getElementById(element.getAttribute('data-expand'))
      if (contents) {
        element.outerHTML = contents.innerHTML
      }
    })
  }
}))

// Light and dark theme toggle
registerHtmxExtension('theme-toggle', () => ({
  init() {
    document.addEventListener('click', (event) => {
      if (!event.target.closest?.('.theme-toggle')) {
        return
      }
      const theme = document.documentElement.getAttribute('data-theme')
      const newTheme = theme === 'dark' ? 'light' : 'dark'
      localStorage.setItem('mojira-theme', newTheme)
      document.documentElement.setAttribute('data-theme', newTheme)
    })
  }
}))

// Expands, highlights, and scrolls to comments in the URL hash
registerHtmxExtension('comment-navigation', () => {
  function expandCommentsIfNeeded() {
    const hash = window.location.hash
    if (hash && hash.startsWith('#comment-')) {
      const element = document.querySelector('[data-expand=hidden-comments]')
      if (element && element.querySelector(hash)) {
        const contents = document.getElementById(element.getAttribute('data-expand'))
        if (contents) {
          element.outerHTML = contents.innerHTML
        }
      }
    }
    document.querySelectorAll('.comment-highlighted').forEach((element) => element.classList.remove('comment-highlighted'))
    if (hash) {
      document.querySelector(hash)?.parentElement?.classList.add('comment-highlighted')
    }
  }

  function onHashChange() {
    expandCommentsIfNeeded()
    const hash = window.location.hash
    if (hash) {
      document.querySelector(hash)?.scrollIntoView({ block: 'start' })
    }
  }

  return {
    init() {
      expandCommentsIfNeeded()
      setTimeout(onHashChange, 500)
      window.addEventListener('hashchange', onHashChange)
    },

    onEvent(name) {
      if (name === 'htmx:afterSwap') {
        expandCommentsIfNeeded()
      }
      return true
    }
  }
})

// Opens image, video, archive, and text attachments in an overlay
registerHtmxExtension('attachment-viewer', () => {
  const DOWNLOAD_ICON = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path d="M2.75 14A1.75 1.75 0 0 1 1 12.25v-2.5a.75.75 0 0 1 1.5 0v2.5c0 .138.112.25.25.25h10.5a.25.25 0 0 0 .25-.25v-2.5a.75.75 0 0 1 1.5 0v2.5A1.75 1.75 0 0 1 13.25 14Z"></path><path d="M7.25 7.689V2a.75.75 0 0 1 1.5 0v5.689l1.97-1.969a.749.749 0 1 1 1.06 1.06l-3.25 3.25a.749.749 0 0 1-1.06 0L4.22 6.78a.749.749 0 1 1 1.06-1.06l1.97 1.969Z"></path></svg>'

  /** @type {HTMLDivElement | null} */
  let overlay

  /**
   * @param {HTMLElement} parent 
   * @param {*} directory 
   */
  function renderArchiveDirectory(parent, directory) {
    const ul = document.createElement('ul')
    const directories = [...directory.directories.entries()].sort(([a], [b]) => a.localeCompare(b))
    const files = directory.files.sort((a, b) => a.name.localeCompare(b.name))

    directories.forEach(([name, child]) => {
      const li = document.createElement('li')
      const details = document.createElement('details')
      const summary = document.createElement('summary')
      summary.textContent = name
      details.append(summary)
      renderArchiveDirectory(details, child)
      li.append(details)
      ul.append(li)
    })

    files.forEach((file) => {
      const li = document.createElement('li')
      li.className = 'file-tree-entry'
      const name = document.createElement('span')
      name.textContent = file.name
      const button = document.createElement('button')
      button.innerHTML = DOWNLOAD_ICON
      button.addEventListener('click', async () => {
        const blob = await file.entry.async('blob')
        const objectUrl = URL.createObjectURL(blob)
        const link = document.createElement('a')
        link.href = objectUrl
        link.download = file.name
        document.body.append(link)
        link.click()
        link.remove()
        setTimeout(() => URL.revokeObjectURL(objectUrl), 1000)
      })
      li.append(name, button)
      ul.append(li)
    })

    if (ul.childElementCount) {
      parent.append(ul)
    }
  }

  /**
   * @param {HTMLElement} tree
   * @param {*} archive
   */
  function renderArchive(tree, archive) {
    tree.replaceChildren()
    const root = { directories: new Map(), files: [] }
    Object.entries(archive.files).forEach(([path, entry]) => {
      path = path.replaceAll('\\', '/')
      const parts = path.split('/').filter(p => p)
      if (!parts.length) {
        return
      }
      let directory = root
      const directoryParts = entry.dir ? parts : parts.slice(0, -1)
      directoryParts.forEach((part) => {
        if (!directory.directories.has(part)) {
          directory.directories.set(part, { directories: new Map(), files: [] })
        }
        directory = directory.directories.get(part)
      })
      if (!entry.dir) {
        directory.files.push({ name: parts.at(-1), path: path, entry })
      }
    })
    renderArchiveDirectory(tree, root)
  }

  /**
   * @param {string} url
   */
  async function loadArchive(url) {
    const { default: JSZip } = await import('https://cdn.jsdelivr.net/npm/jszip@3.10.1/+esm')
    const response = await fetch(url)
    const buffer = await response.arrayBuffer()
    return JSZip.loadAsync(buffer)
  }

  /**
   * @param {string} id
   * @returns {HTMLElement | null}
   */
  function findAttachment(id) {
    return document.querySelector(`[data-attachment="${id}"]`)
  }

  /**
   * @param {HTMLElement | null} el
   * @returns {boolean}
   */
  function showAttachment(el) {
    if (!overlay || !el) {
      return false
    }
    const fileType = el.getAttribute('data-attachment-type')
    if (fileType === 'unknown') {
      return false
    }
    const fileInfo = el.getAttribute('data-attachment-info')
    const fileUrl = el.getAttribute('href')
    const image = overlay.querySelector('img')
    const video = overlay.querySelector('video')
    const text = overlay.querySelector('textarea')
    const fileTree = overlay.querySelector('.file-tree')
    image.src = ''
    video.src = ''
    text.value = ''
    fileTree.replaceChildren()
    const id = el.getAttribute('data-attachment')
    if (fileType === 'image') {
      image.src = fileUrl
      image.alt = fileInfo
    } else if (fileType === 'video') {
      video.src = fileUrl
      video.alt = fileInfo
      video.controls = true
    } else if (fileType === 'archive') {
      fileTree.replaceChildren()
      const msg = document.createElement('span')
      msg.textContent = 'Loading archive...'
      fileTree.append(msg)
      loadArchive(fileUrl)
        .then(archive => {
          if (overlay.getAttribute('data-current-id') === id) {
            renderArchive(fileTree, archive)
          }
        })
        .catch(error => {
          console.error(`Failed loading archive ${id}:`, error)
          if (overlay.getAttribute('data-current-id') === id) {
            fileTree.replaceChildren()
            const msg = document.createElement('span')
            msg.textContent = 'Failed to load archive.'
            fileTree.append(msg)
          }
        })
    } else if (fileType === 'text') {
      fetch(fileUrl)
        .then(res => res.text())
        .then(contents => {
          if (overlay.getAttribute('data-current-id') === id) {
            text.value = contents
          }
        })
    } else {
      return false
    }
    overlay.querySelector('.file-overlay-info').href = fileUrl
    overlay.querySelector('.file-overlay-name').textContent = fileInfo
    overlay.setAttribute('data-current-id', id)
    overlay.setAttribute('data-file-type', fileType)
    const url = new URL(window.location)
    url.searchParams.set('attachment', id)
    window.history.replaceState({}, '', url)
    return true
  }

  function closeOverlay() {
    if (overlay) {
      overlay.querySelector('video').src = ''
      overlay.querySelector('img').src = ''
      overlay.querySelector('.file-tree').replaceChildren()
      overlay.querySelector('textarea').value = ''
      overlay.removeAttribute('data-current-id')
      overlay.removeAttribute('data-file-type')
    }
    const url = new URL(window.location)
    url.searchParams.delete('attachment')
    window.history.replaceState({}, '', url)
  }

  /**
   * @param {string} direction
   */
  function moveAttachment(direction) {
    if (!overlay) {
      return
    }
    const currentEl = findAttachment(overlay.getAttribute('data-current-id'))
    let el = currentEl?.[direction === 'prev' ? 'previousElementSibling' : 'nextElementSibling']
    while (el && el.getAttribute('data-attachment-type') === 'unknown') {
      el = el[direction === 'prev' ? 'previousElementSibling' : 'nextElementSibling']
    }
    showAttachment(el)
  }

  return {
    init() {
      overlay = document.getElementById('file-overlay')
      document.addEventListener('click', (event) => {
        const attachment = event.target.closest?.('[data-attachment]')
        if (attachment) {
          if (event.ctrlKey || event.shiftKey) {
            return
          }
          if (showAttachment(attachment)) {
            event.preventDefault()
          }
          return
        }
        if (event.target.closest?.('.file-overlay-backdrop')) {
          closeOverlay()
          return
        }
        if (event.target.closest?.('.file-overlay-arrow.left')) {
          moveAttachment('prev')
        } else if (event.target.closest?.('.file-overlay-arrow.right')) {
          moveAttachment('next')
        }
      })

      document.addEventListener('keydown', (event) => {
        if (!overlay?.hasAttribute('data-current-id')) {
          return
        }
        if (event.key === 'Escape') closeOverlay()
        if (event.key === 'ArrowLeft') moveAttachment('prev')
        if (event.key === 'ArrowRight') moveAttachment('next')
      })

      const attachment = new URL(window.location).searchParams.get('attachment')
      if (attachment) {
        showAttachment(findAttachment(attachment))
      }
    }
  }
})

// Make table columns resizable
registerHtmxExtension('table-resize', () => {
  /**
   * @param {number} x
   * @param {number} y
   * @returns {HTMLTableCellElement | undefined}
   */
  function findHandle(x, y) {
    let nearestHeader
    let nearestDistance = Infinity
    document.querySelectorAll('th[data-resizable]').forEach((header) => {
      const bounds = header.getBoundingClientRect()
      const distance = Math.abs(bounds.right - x)
      if (y >= bounds.top && y <= bounds.bottom && distance <= 8 && distance < nearestDistance) {
        nearestHeader = header
        nearestDistance = distance
      }
    })
    return nearestHeader
  }

  /**
   * @param {HTMLTableElement} table
   * @param {number} index
   * @param {number} width
   */
  function setColumnWidth(table, index, width) {
    Array.from(table.rows).forEach((row) => {
      const cell = row.cells[index]
      if (cell) {
        cell.style.width = `${width}px`
        if (cell.firstElementChild) {
          cell.firstElementChild.style.width = `${width}px`
        }
      }
    })
  }

  /**
   * @param {Element} root
   */
  function restoreWidths(root) {
    const tables = new Set(root.matches?.('table') ? [root] : [])
    if (root.closest) {
      const parentTable = root.closest('table')
      if (parentTable) {
        tables.add(parentTable)
      }
    }
    root.querySelectorAll('table').forEach((table) => tables.add(table))
    tables.forEach((table) => {
      table.querySelectorAll('th[data-resizable]').forEach((header) => {
        const key = header.getAttribute('data-resizable')
        const width = Number(sessionStorage.getItem(`resize-${key}`))
        if (Number.isFinite(width) && width >= 80) {
          setColumnWidth(table, header.cellIndex, width)
        }
      })
    })
  }

  function endResize() {
    const root = document.documentElement
    root.removeAttribute('data-resizable-key')
    root.removeAttribute('data-resizable-start-x')
    root.removeAttribute('data-resizable-start-width')
    root.style.cursor = ''
  }

  return {
    init() {
      restoreWidths(document)

      document.addEventListener('pointerdown', (event) => {
        if (event.button !== 0) {
          return
        }
        const header = findHandle(event.clientX, event.clientY)
        if (!header) {
          return
        }
        const root = document.documentElement
        root.setAttribute('data-resizable-key', header.getAttribute('data-resizable'))
        root.setAttribute('data-resizable-start-x', event.clientX)
        root.setAttribute('data-resizable-start-width', getComputedStyle(header.firstElementChild || header).width)
        event.preventDefault()
      })

      document.addEventListener('pointermove', (event) => {
        const root = document.documentElement
        const key = root.getAttribute('data-resizable-key')
        if (key === null) {
          root.style.cursor = findHandle(event.clientX, event.clientY) ? 'col-resize' : ''
          return
        }
        root.style.cursor = 'col-resize'
        const startX = Number(root.getAttribute('data-resizable-start-x'))
        const startWidth = parseFloat(root.getAttribute('data-resizable-start-width'))
        const width = Math.max(80, Math.round(startWidth + event.clientX - startX))
        document.querySelectorAll('th[data-resizable]').forEach((header) => {
          if (header.getAttribute('data-resizable') === key) {
            setColumnWidth(header.closest('table'), header.cellIndex, width)
          }
        })
        sessionStorage.setItem(`resize-${key}`, width)
      })

      document.addEventListener('pointerup', endResize)
      document.addEventListener('pointercancel', endResize)
    },

    onEvent(name, event) {
      if (name === 'htmx:afterSwap') {
        restoreWidths(event.target)
      }
      return true
    }
  }
})
