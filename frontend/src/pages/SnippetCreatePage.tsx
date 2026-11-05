import { Link, useNavigate } from 'react-router-dom'
import SnippetForm from '@/components/SnippetForm'

export default function SnippetCreatePage() {
  const navigate = useNavigate()

  return (
    <>
      <p className="mb-4 inline-block text-sm">
        <Link to="/snippets" className="no-underline hover:underline">
          ← Back to Cover Letter Snippets
        </Link>
      </p>
      <h1>New Cover Letter Snippet</h1>
      <SnippetForm onCreated={(created) => navigate(`/snippets/${created.id}`)} />
    </>
  )
}
