import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import 'react-grid-layout/css/styles.css';
import 'react-resizable/css/styles.css';
import 'leaflet/dist/leaflet.css';
import './index.css'
import { Toaster } from 'sonner' 

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
    <Toaster position="top-right" theme="dark" richColors closeButton />
  </React.StrictMode>,
)