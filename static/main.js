function afterSwap() {
  document.querySelectorAll('time[datetime]').forEach((el) => {
    const d = new Date(el.getAttribute('datetime'))
    if (!isNaN(d)) {
      el.textContent = d.toLocaleString()
    }
  })

  document.querySelectorAll('[data-copy]').forEach((el) => {
    let copiedTimer
    el.addEventListener('click', () => {
      const value = el.getAttribute('data-copy')
      navigator.clipboard.writeText(value)
      el.classList.add('copied')
      clearTimeout(copiedTimer)
      copiedTimer = setTimeout(() => el.classList.remove('copied'), 3000)
    })
  })
}

afterSwap()

document.body.addEventListener('htmx:afterSwap', () => {
  afterSwap()
})
