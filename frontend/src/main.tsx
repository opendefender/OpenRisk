// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import 'react-grid-layout/css/styles.css';
import 'react-resizable/css/styles.css';
import 'leaflet/dist/leaflet.css';
import './index.css'
import { Toaster } from 'sonner'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useUIStore } from './store/uiStore'

const queryClient = new QueryClient()

/** Toasts follow the active theme (dc.html §8). */
function ThemedToaster() {
  const theme = useUIStore((s) => s.theme)
  return <Toaster position="top-right" theme={theme} richColors closeButton />
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
      <ThemedToaster />
    </QueryClientProvider>
  </React.StrictMode>,
)