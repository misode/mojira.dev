document.querySelectorAll('time[datetime]').forEach((el) => {
  const d = new Date(el.getAttribute('datetime'))
  if (!isNaN(d)) {
    el.textContent = d.toLocaleString()
  }
})
