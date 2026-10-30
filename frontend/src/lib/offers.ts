import type { Offer, RALRange } from '@/api/types'

export type OfferSort = 'ral' | 'date'

export interface OfferGroup {
  /** Currency code, or '' for offers with no RAL figure to compare. */
  currency: string
  offers: Offer[]
}

/** Highest figure the RAL Range declares, or undefined when it names none or is a conflict (no figure is picked). */
export function ralMax(ral: RALRange): number | undefined {
  if (ral.source === 'conflict' || ral.source === 'n/a') return undefined
  return ral.max ?? undefined
}

// A conflict is grouped under its currency when both declared figures agree
// on one, and with the offers that have no comparable figure otherwise.
function groupCurrency(ral: RALRange): string {
  if (ral.source === 'conflict') {
    const a = ral.descriptionStated?.currency
    const b = ral.listingStated?.currency
    return a && a === b ? a : ''
  }
  return ral.max != null && ral.source !== 'n/a' ? (ral.currency ?? '') : ''
}

function byDate(a: Offer, b: Offer): number {
  // ISO timestamps sort lexicographically; undated offers go last.
  if (!a.offeredAt || !b.offeredAt) return a.offeredAt ? -1 : b.offeredAt ? 1 : 0
  return a.offeredAt < b.offeredAt ? 1 : a.offeredAt > b.offeredAt ? -1 : 0
}

/**
 * Groups offers by currency (amounts in different currencies are never
 * sorted together nor converted) and sorts each group by highest RAL or by
 * date. Offers without a single comparable figure (conflict, unknown) follow
 * the ones with a figure, newest first. Pure: no conversion, no guessing.
 */
export function groupOffers(offers: Offer[], sort: OfferSort): OfferGroup[] {
  const groups = new Map<string, Offer[]>()
  for (const offer of offers) {
    const key = groupCurrency(offer.ral)
    groups.set(key, [...(groups.get(key) ?? []), offer])
  }
  const compare = (a: Offer, b: Offer): number => {
    if (sort === 'ral') {
      const ma = ralMax(a.ral)
      const mb = ralMax(b.ral)
      if (ma !== mb) {
        if (ma === undefined) return 1
        if (mb === undefined) return -1
        return mb - ma
      }
    }
    return byDate(a, b)
  }
  return [...groups.entries()]
    .sort(([a], [b]) => (a === '' ? 1 : b === '' ? -1 : a.localeCompare(b)))
    .map(([currency, list]) => ({ currency, offers: [...list].sort(compare) }))
}
