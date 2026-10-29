import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getOffers } from '@/api/client'
import type { Offer } from '@/api/types'
import RALBadge from '@/components/RALBadge'
import { groupOffers, type OfferSort } from '@/lib/offers'

function offerHeading(offer: Offer): string {
  return offer.title ? `${offer.company} — ${offer.title}` : offer.company
}

function dateLabel(iso?: string): string {
  if (!iso) return 'Offer date unknown'
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? 'Offer date unknown' : `Offer received ${d.toLocaleDateString()}`
}

function OfferColumn({ offer }: { offer: Offer }) {
  return (
    <li className="flex min-w-64 flex-1 flex-col gap-2 rounded-xl border border-border bg-card px-4 py-3">
      <Link to={`/jobs/${encodeURIComponent(offer.id)}`} className="font-semibold no-underline hover:underline">
        {offerHeading(offer)}
      </Link>
      <span className="text-sm text-muted-foreground">{offer.location || 'Location not stated'}</span>
      <span className="text-sm">{dateLabel(offer.offeredAt)}</span>
      <RALBadge ral={offer.ral} />
      {offer.notes.length > 0 && (
        <ul className="flex flex-col gap-1 text-sm">
          {offer.notes.map((note) => (
            <li key={note.id} className="whitespace-pre-wrap border-l-2 border-border pl-2">
              {note.body}
            </li>
          ))}
        </ul>
      )}
    </li>
  )
}

export default function OffersPage() {
  const [offers, setOffers] = useState<Offer[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [sort, setSort] = useState<OfferSort>('ral')

  useEffect(() => {
    getOffers()
      .then(setOffers)
      .catch((e) => setError(e.message))
  }, [])

  if (error)
    return (
      <p role="alert" className="font-medium text-destructive">
        {error}
      </p>
    )
  if (!offers) return <p>Loading…</p>

  const groups = groupOffers(offers, sort)

  return (
    <>
      <h1>Offers</h1>
      {offers.length === 0 ? (
        <p>No offers yet — an Application moved to Offer will show up here.</p>
      ) : (
        <>
          <label className="mb-4 flex items-center gap-2 text-sm">
            Sort by
            <select value={sort} onChange={(e) => setSort(e.target.value as OfferSort)}>
              <option value="ral">Highest RAL</option>
              <option value="date">Offer date</option>
            </select>
          </label>
          {groups.length > 1 && (
            <p role="note" className="mb-4 text-sm text-muted-foreground">
              These offers are in different currencies. They are not converted and not comparable with each other, so
              each currency is listed on its own.
            </p>
          )}
          {groups.map((group) => {
            const title = group.currency || 'No single RAL figure'
            return (
              <section key={group.currency} aria-label={title} className="mb-8">
                <h2>{title}</h2>
                <ul className="flex flex-wrap items-stretch gap-4">
                  {group.offers.map((offer) => (
                    <OfferColumn key={offer.id} offer={offer} />
                  ))}
                </ul>
              </section>
            )
          })}
        </>
      )}
    </>
  )
}
