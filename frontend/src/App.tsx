import { BrowserRouter, Routes, Route, Navigate, Outlet } from 'react-router-dom';
import { hasToken } from './api';
import { Layout } from './components/Layout';
import { Login } from './pages/Login';

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
          <Route path="/dashboard" element={<div />} />
          <Route path="/projects" element={<div />} />
          <Route path="/projects/:id" element={<div />} />
          <Route path="/settings" element={<div />} />
          <Route path="/plugins" element={<div />} />
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
