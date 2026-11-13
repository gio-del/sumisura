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
          <div className="md:flex">
            <AppNav />
            <main className="mx-auto min-w-0 max-w-[960px] flex-1 px-6 pt-6 pb-[calc(6rem+env(safe-area-inset-bottom))] sm:px-4 md:pb-12">
              <AppRoutes />
            </main>
          </div>
        </BrowserRouter>
      </AccessGate>
    </TooltipProvider>
  )
}

export default App
