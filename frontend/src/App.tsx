import { BrowserRouter, Routes, Route, Navigate, Outlet } from 'react-router-dom';
import { hasToken } from './api';
import { Layout } from './components/Layout';
import { Login } from './pages/Login';
import Dashboard from './pages/Dashboard';
import Projects from './pages/Projects';
import Settings from './pages/Settings';
import Plugins from './pages/Plugins';
import ProjectDetail from './project/ProjectDetail';

function AuthGuard() {
  if (!hasToken()) {
    return <Navigate to="/" replace />;
  }
  return <Outlet />;
}

function GuestGuard() {
  if (hasToken()) {
    return <Navigate to="/dashboard" replace />;
  }
  return <Login />;
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/" element={<GuestGuard />} />

      <Route element={<AuthGuard />}>
        <Route element={<Layout />}>
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/projects" element={<Projects />} />
          <Route path="/projects/:id" element={<ProjectDetail />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="/plugins" element={<Plugins />} />
        </Route>
      </Route>

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <AppRoutes />
    </BrowserRouter>
  );
}
