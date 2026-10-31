import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getTrash, purgeTrashItem, restoreTrashItem } from '@/api/client'
import type { TrashItem } from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export default function TrashPage() {
  const [items, setItems] = useState<TrashItem[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [restored, setRestored] = useState<{ id: string; kind: TrashItem['kind'] } | null>(null)

  const load = useCallback(() => {
    getTrash()
      .then((list) => setItems(list.items))
      .catch((e) => setError(e.message))
  }, [])

  useEffect(load, [load])

  async function act(run: () => Promise<void>) {
    setError(null)
    try {
      await run()
    } catch (e) {
      // The item may be gone already (purged by its retention in another tab);
      // reload so the list shows what is really there.
      setError(e instanceof Error ? e.message : String(e))
    }
    load()
  }

  if (!items) {
    return error ? (
      <p role="alert" className="font-medium text-destructive">
        {error}
      </p>
    ) : (
      <p>Loading…</p>
    )
  }

  return (
    <>
      <div className="mb-6">
        <h1 className="mb-1">Trash</h1>
        <p className="mb-0 text-muted-foreground">
          The previous version of an Entry or Cover Letter Snippet you deleted or overwrote. The last 10 versions of
          each item are kept, for 90 days. Restoring brings it back under its old id, or a numbered one if that id is
          taken.
        </p>
      </div>

      {error && (
        <p role="alert" className="font-medium text-destructive">
          {error}
        </p>
      )}
      {restored && (
        <p role="status">
          Restored as{' '}
          <Link to={restored.kind === 'snippet' ? `/snippets/${restored.id}` : `/entries/${restored.id}`}>
            {restored.id}
          </Link>
          .
        </p>
      )}

      {items.length === 0 && <p>The trash is empty.</p>}

      <div className="flex flex-col gap-3">
        {items.map((item) => (
          <Card key={item.id}>
            <CardHeader>
              <CardTitle>
                {item.label}{' '}
                <Badge variant="secondary">{item.kind === 'snippet' ? 'Cover Letter Snippet' : 'Entry'}</Badge>{' '}
                <span className="text-sm font-normal text-muted-foreground">
                  {item.originalId}, trashed {new Date(item.trashedAt).toLocaleString()}
                </span>
              </CardTitle>
            </CardHeader>
            <CardContent>
              <details className="mb-3">
                <summary>Content</summary>
                <pre className="whitespace-pre-wrap text-sm">{item.content}</pre>
              </details>
              <div className="flex gap-2">
                <Button
                  type="button"
                  onClick={() =>
                    act(async () => {
                      setRestored(await restoreTrashItem(item.id))
                    })
                  }
                >
                  Restore
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() =>
                    act(async () => {
                      await purgeTrashItem(item.id)
                    })
                  }
                >
                  Delete permanently
                </Button>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </>
  )
}
