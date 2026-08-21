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
import { ErrorBoundary } from './shared/system/ErrorBoundary'
import { installGlobalErrorReporting } from './lib/observability'
import { registerQueryClient } from './lib/sessionScope'

// Unstable-connectivity tolerance (task §2). Queries are offline-first: they serve
// the cached value immediately and revalidate when the network allows (SWR), and
// the cache is kept long enough to survive flaky links. Mutations use the default
// online network mode, so a write attempted while offline is PAUSED and replayed
// automatically on reconnect — a built-in mutation queue. The OfflineBanner shows
// the state and how many writes are waiting.
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      networkMode: 'offlineFirst',
      staleTime: 30_000,
      gcTime: 24 * 60 * 60 * 1000, // keep cache a day so a reload offline still paints
      retry: 2,
      refetchOnReconnect: true,
    },
    mutations: {
      networkMode: 'online',
      retry: 3,
      retryDelay: (attempt) => Math.min(1000 * 2 ** attempt, 15_000),
    },
  },
})

// Hand the cache to the session scope, so signing out (or in) can empty it.
// Without this the tab keeps one tenant's responses across a change of user —
// logout and login are both soft navigations, so nothing else tears the cache
// down (W0-05 / D9).
registerQueryClient(queryClient)

// Report uncaught errors and unhandled rejections (Sentry when present).
installGlobalErrorReporting()

/** Toasts follow the active theme (dc.html §8). */
function ThemedToaster() {
  const theme = useUIStore((s) => s.theme)
  return <Toaster position="top-right" theme={theme} richColors closeButton />
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <ErrorBoundary>
        <App />
      </ErrorBoundary>
      <ThemedToaster />
    </QueryClientProvider>
  </React.StrictMode>,
)