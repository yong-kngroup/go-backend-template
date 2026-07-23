const toggle = document.querySelector("[data-theme-toggle]");
const label = document.querySelector("[data-theme-label]");

function currentTheme() {
  if (document.documentElement.dataset.theme) return document.documentElement.dataset.theme;
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function updateThemeControl() {
  if (!toggle) return;
  const dark = currentTheme() === "dark";
  const nextLabel = dark ? toggle.dataset.labelLight : toggle.dataset.labelDark;
  toggle.setAttribute("aria-label", nextLabel);
  toggle.setAttribute("title", nextLabel);
  if (label) label.textContent = nextLabel;
  toggle.innerHTML = `<i data-lucide="${dark ? "sun" : "moon"}" aria-hidden="true"></i><span class="sr-only" data-theme-label>${nextLabel}</span>`;
  window.lucide?.createIcons();
}

toggle?.addEventListener("click", () => {
  const next = currentTheme() === "dark" ? "light" : "dark";
  document.documentElement.dataset.theme = next;
  try {
    localStorage.setItem("site-theme", next);
  } catch {
    // The in-memory selection still applies for this page.
  }
  updateThemeControl();
});

window.lucide?.createIcons();
updateThemeControl();
