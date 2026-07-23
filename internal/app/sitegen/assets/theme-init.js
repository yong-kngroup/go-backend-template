try {
  const theme = localStorage.getItem("site-theme");
  if (theme === "light" || theme === "dark") {
    document.documentElement.dataset.theme = theme;
  }
} catch {
  // System preference remains the fallback when storage is unavailable.
}
