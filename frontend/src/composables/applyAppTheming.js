export function applyAppTheming(features) {
  if (features?.themeName) {
    document.documentElement.dataset.theme = features.themeName
  }
}
