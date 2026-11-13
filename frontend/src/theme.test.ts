import { describe, expect, it } from 'vitest'
import css from './index.css?raw'
import buttonSource from './components/ui/button.tsx?raw'
import { allowedNextStatuses, statusBadgeClass } from '@/lib/applicationStatus'

// The theme lives in CSS no component test renders, so its guarantees are
// pinned from the stylesheet itself: every Status badge and the base text
// pairs stay >= 4.5:1 in the light and the dark theme (issue #305).

function vars(block: string): Record<string, string> {
  const out: Record<string, string> = {}
  for (const m of block.matchAll(/(--[a-z0-9-]+):\s*(#[0-9a-f]{6})\s*;/gi)) out[m[1]] = m[2].toLowerCase()
  return out
}

const darkStart = css.indexOf('@media (prefers-color-scheme: dark)')
const light = vars(css.slice(css.indexOf(':root {'), darkStart))
const dark = { ...light, ...vars(css.slice(darkStart, css.indexOf('/* Visible keyboard focus')) ) }

function luminance(hex: string): number {
  const ch = [1, 3, 5].map((i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * ch[0] + 0.7152 * ch[1] + 0.0722 * ch[2]
}
function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

const themes = { light, dark }
const statuses = Object.keys(allowedNextStatuses)

describe('theme', () => {
  it('Palette_MatchesTheApprovedValues', () => {
    expect(light['--background']).toBe('#f6f3ec')
    expect(light['--card']).toBe('#fffefb')
    expect(light['--accent']).toBe('#e5a712')
    expect(dark['--background']).toBe('#12172a')
    expect(dark['--primary']).toBe('#f2c24a')
    expect(dark['--primary-foreground']).toBe('#12172a')
  })

  for (const [name, t] of Object.entries(themes)) {
    it(`StatusBadges_ContrastAtLeast4_5_${name}`, () => {
      for (const s of statuses) {
        expect(t[`--status-${s}-bg`], s).toBeDefined()
        expect(contrast(t[`--status-${s}-bg`], t[`--status-${s}-fg`]), `${s} (${name})`).toBeGreaterThanOrEqual(4.5)
      }
    })

    it(`BaseText_ContrastAtLeast4_5_${name}`, () => {
      const pairs: [string, string][] = [
        ['--foreground', '--background'],
        ['--card-foreground', '--card'],
        ['--muted-foreground', '--background'],
        ['--muted-foreground', '--card'],
        ['--muted-foreground', '--muted'],
        ['--primary-foreground', '--primary'],
        ['--ok', '--card'],
        ['--warning', '--card'],
        ['--destructive', '--card'],
      ]
      for (const [fg, bg] of pairs) expect(contrast(t[fg], t[bg]), `${fg} on ${bg} (${name})`).toBeGreaterThanOrEqual(4.5)
    })
  }

  it('StatusBadges_EachStatusHasItsOwnColour', () => {
    const classes = statuses.map((s) => statusBadgeClass[s as keyof typeof statusBadgeClass])
    expect(new Set(classes).size).toBe(statuses.length)
  })

  it('Buttons_AllSizesAre44pxOrMore', () => {
    expect(buttonSource).not.toMatch(/\bh-[5-9]\b|\bsize-[5-9]\b/)
    expect(buttonSource.match(/min-h-11|size-11/g)?.length).toBeGreaterThanOrEqual(8)
  })

  it('Fonts_AreSystemOnly', () => {
    expect(css).not.toMatch(/Inter|Instrument|fontsource/)
  })
})
