import { describe, expect, it } from 'vitest'
import manifestSource from '../public/manifest.webmanifest?raw'
import serviceWorkerSource from '../public/sw.js?raw'
import indexHtml from '../index.html?raw'

// Raw imports keep this test inside the app's own type setup (no Node
// types): the files are read through Vite exactly as they ship.
const publicFiles = new Set(
  Object.keys(import.meta.glob('../public/**/*.png', { eager: false })).map((path) => path.replace('../public', '')),
)

interface Manifest {
  name: string
  start_url: string
  display: string
  icons: { src: string; sizes: string; purpose?: string }[]
  share_target: { action: string; method: string; enctype: string; params: Record<string, string> }
}

// Installability and the share target (issue #185) live in static files no
// component test reaches, so their essentials are pinned here.
describe('PWA manifest', () => {
  const manifest = JSON.parse(manifestSource) as Manifest

  it('Manifest_HasWhatChromeNeedsToInstall', () => {
    expect(manifest.name).toBe('Sumisura')
    expect(manifest.start_url).toBe('/')
    expect(manifest.display).toBe('standalone')
    const sizes = manifest.icons.filter((i) => (i.purpose ?? 'any') === 'any').map((i) => i.sizes)
    expect(sizes).toEqual(expect.arrayContaining(['192x192', '512x512']))
    expect(manifest.icons.some((i) => i.purpose === 'maskable')).toBe(true)
  })

  it('Manifest_EveryIconFileExists', () => {
    for (const icon of manifest.icons) {
      expect(publicFiles.has(icon.src), icon.src).toBe(true)
    }
  })

  it('Manifest_SharesArriveAsGetParamsOnTheSharePage', () => {
    expect(manifest.share_target).toEqual({
      action: '/share',
      method: 'GET',
      enctype: 'application/x-www-form-urlencoded',
      params: { title: 'title', text: 'text', url: 'url' },
    })
  })

  it('IndexHtml_LinksManifestAndServiceWorkerExists', () => {
    expect(indexHtml).toContain('<link rel="manifest" href="/manifest.webmanifest" />')
    expect(indexHtml).toContain('<link rel="apple-touch-icon" href="/icons/apple-touch-icon.png" />')
    expect(publicFiles.has('/icons/apple-touch-icon.png')).toBe(true)
    expect(serviceWorkerSource).toContain('addEventListener("fetch"')
  })
})
