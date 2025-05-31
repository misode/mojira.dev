document.querySelectorAll('time[datetime]').forEach((el) => {
  const d = new Date(el.getAttribute('datetime'))
  if (!isNaN(d)) {
    el.textContent = d.toLocaleString()
  }
})

const searchInput = document.getElementById('search-input')
searchInput.addEventListener(async () => {
  const response = fetch('/api/search', {
    method: 'POST',
    body: JSON.stringify({query: searchInput.value}),
  })
  
})
