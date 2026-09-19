import { Route, Routes } from 'react-router-dom'
import ApplicationsPage from '@/pages/ApplicationsPage'
import AtsBrowsePage from '@/pages/AtsBrowsePage'
import EntriesListPage from '@/pages/EntriesListPage'
import EntryCreatePage from '@/pages/EntryCreatePage'
import EntryDetailPage from '@/pages/EntryDetailPage'
import GenerationPage from '@/pages/GenerationPage'
import ATSReportPage from '@/pages/ATSReportPage'
import GenerationsListPage from '@/pages/GenerationsListPage'
import JobListingCreatePage from '@/pages/JobListingCreatePage'
import JobListingDetailPage from '@/pages/JobListingDetailPage'
import JobListingsListPage from '@/pages/JobListingsListPage'
import PendingCapturesPage from '@/pages/PendingCapturesPage'
import ProfilePage from '@/pages/ProfilePage'
import SharePage from '@/pages/SharePage'
import SnippetCreatePage from '@/pages/SnippetCreatePage'
import SnippetDetailPage from '@/pages/SnippetDetailPage'
import SnippetsListPage from '@/pages/SnippetsListPage'
import StatsPage from '@/pages/StatsPage'
import TagLintPage from '@/pages/TagLintPage'

// AppRoutes is the app's route table, kept apart from App's BrowserRouter so
// tests can mount the real table inside a MemoryRouter. React Router ranks a
// static segment above a dynamic one, so /jobs/new still wins over /jobs/:id,
// and /jobs/:id/generate stays distinct from both (issue #94).
export default function AppRoutes() {
  return (
    <Routes>
      <Route path="/" element={<EntriesListPage />} />
      <Route path="/entries/new" element={<EntryCreatePage />} />
      <Route path="/tags" element={<TagLintPage />} />
      <Route path="/entries/*" element={<EntryDetailPage />} />
      <Route path="/profile" element={<ProfilePage />} />
      <Route path="/generate" element={<GenerationPage />} />
      <Route path="/generations" element={<GenerationsListPage />} />
      <Route path="/generations/:slug/ats" element={<ATSReportPage />} />
      <Route path="/jobs" element={<JobListingsListPage />} />
      <Route path="/inbox" element={<PendingCapturesPage />} />
      <Route path="/share" element={<SharePage />} />
      <Route path="/applications" element={<ApplicationsPage />} />
      <Route path="/stats" element={<StatsPage />} />
      <Route path="/jobs/new" element={<JobListingCreatePage />} />
      <Route path="/jobs/:id" element={<JobListingDetailPage />} />
      <Route path="/ats" element={<AtsBrowsePage />} />
      <Route path="/jobs/:id/generate" element={<GenerationPage />} />
      <Route path="/snippets" element={<SnippetsListPage />} />
      <Route path="/snippets/new" element={<SnippetCreatePage />} />
      <Route path="/snippets/:id" element={<SnippetDetailPage />} />
    </Routes>
  )
}
