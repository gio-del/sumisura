import { BrowserRouter } from 'react-router-dom'
import AppRoutes from '@/AppRoutes'
import AccessGate from '@/components/AccessGate'
import AppNav from '@/components/AppNav'
import { TooltipProvider } from '@/components/ui/tooltip'

function App() {
  return (
    <TooltipProvider>
      <AccessGate>
        <BrowserRouter>
          <AppNav />
          <main className="mx-auto max-w-[960px] px-6 pt-6 pb-12 sm:px-4">
            <AppRoutes />
          </main>
        </BrowserRouter>
      </AccessGate>
    </TooltipProvider>
  )
}

export default App
