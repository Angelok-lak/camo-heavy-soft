// Thèmes d'affichage. Pur confort visuel côté client : aucune donnée
// métier, la sévérité des conflits garde les mêmes codes partout.
export const THEMES = [
  { id: '', label: 'Sable' },
  { id: 'craie', label: 'Craie' },
  { id: 'ardoise', label: 'Ardoise' },
  { id: 'lagune', label: 'Lagune' },
  { id: 'nuit', label: 'Nuit' },
] as const

export type ThemeId = (typeof THEMES)[number]['id']

const KEY = 'camo-theme'

export function applyTheme(id: string) {
  if (id) document.documentElement.dataset.theme = id
  else delete document.documentElement.dataset.theme
  try {
    localStorage.setItem(KEY, id)
  } catch {
    /* stockage indisponible : le thème vaut pour la session en cours */
  }
}

export function savedTheme(): string {
  try {
    const id = localStorage.getItem(KEY) ?? ''
    return THEMES.some((t) => t.id === id) ? id : ''
  } catch {
    return ''
  }
}

export function initTheme() {
  applyTheme(savedTheme())
}
