document.querySelectorAll('time[datetime]').forEach(function(el) {
  const d = new Date(el.getAttribute('datetime'))
  if (!isNaN(d)) {
    el.textContent = d.toLocaleString()
  }
})
