import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { RunPage } from './pages/RunPage'
import { RunsPage } from './pages/RunsPage'

const root = document.getElementById('root')
if (!root) throw new Error('#root не найден')

createRoot(root).render(
  <StrictMode>
    <BrowserRouter>
      <main style={{ fontFamily: 'system-ui, sans-serif', maxWidth: 1100, margin: '0 auto', padding: 16 }}>
        <Routes>
          <Route path="/" element={<RunsPage />} />
          <Route path="/runs/:id" element={<RunPage />} />
          <Route path="*" element={<p>Страница не найдена. <a href="/">К запускам</a></p>} />
        </Routes>
      </main>
    </BrowserRouter>
  </StrictMode>,
)
