import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { Layout } from './components/Layout';
import { Overview } from './pages/Overview';
import { ExperimentsList } from './pages/ExperimentsList';
import { ExperimentDetail } from './pages/ExperimentDetail';
import { Live } from './pages/Live';
import { Suites } from './pages/Suites';
import { Operators } from './pages/Operators';
import { Knowledge } from './pages/Knowledge';

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<Overview />} />
          <Route path="live" element={<Live />} />
          <Route path="experiments" element={<ExperimentsList />} />
          <Route path="experiments/:namespace/:name" element={<ExperimentDetail />} />
          <Route path="suites" element={<Suites />} />
          <Route path="operators" element={<Operators />} />
          <Route path="knowledge" element={<Knowledge />} />
          <Route path="*" element={<div style={{padding:24}}><h2>Page not found</h2></div>} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
