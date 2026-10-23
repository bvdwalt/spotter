const plane = (fill: string) =>
  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" width="64" height="64">
  <path fill="${fill}" stroke="#111" stroke-width="2" stroke-linejoin="round"
    d="M32 3 L36 8 L37 26 L60 40 L60 46 L37 38 L36 52 L43 57 L43 61 L32 58 L21 61 L21 57 L28 52 L27 38 L4 46 L4 40 L27 26 L28 8 Z"/>
</svg>`;

const ship = (fill: string) =>
  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" width="64" height="64">
  <path fill="${fill}" stroke="#111" stroke-width="2" stroke-linejoin="round"
    d="M32 4 L46 24 L46 58 L18 58 L18 24 Z"/>
</svg>`;

export const ICONS: Record<string, string> = {
  plane: plane("#ffffff"),
  "plane-ground": plane("#8a8a8a"),
  "plane-military": plane("#ff9f43"),
  "plane-emergency": plane("#ff4757"),
  "plane-selected": plane("#4ecdc4"),
  ship: ship("#74b9ff"),
  "ship-selected": ship("#4ecdc4"),
};

export function loadIcon(svg: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image(64, 64);
    img.onload = () => resolve(img);
    img.onerror = reject;
    img.src = "data:image/svg+xml;charset=utf-8," + encodeURIComponent(svg);
  });
}
