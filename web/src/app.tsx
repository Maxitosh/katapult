// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1

import { BrowserRouter, Routes, Route } from "react-router-dom"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { AppLayout } from "@/components/layout/app-layout"
import { TransfersPage } from "@/pages/transfers-page"
import { TransferDetailPage } from "@/pages/transfer-detail-page"
import { CreateTransferPage } from "@/pages/create-transfer-page"
import { AgentsPage } from "@/pages/agents-page"
import { AgentDetailPage } from "@/pages/agent-detail-page"

const queryClient = new QueryClient()

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route element={<AppLayout />}>
            <Route path="/" element={<TransfersPage />} />
            <Route path="/transfers/new" element={<CreateTransferPage />} />
            <Route path="/transfers/:id" element={<TransferDetailPage />} />
            <Route path="/agents" element={<AgentsPage />} />
            <Route path="/agents/:id" element={<AgentDetailPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
