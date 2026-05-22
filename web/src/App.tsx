import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { HomePage } from './pages/HomePage'
import { ResultPage } from './pages/ResultPage'

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/backtests/:id" element={<ResultPage />} />
      </Routes>
    </BrowserRouter>
  )
}
