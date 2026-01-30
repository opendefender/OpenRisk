import axios from 'axios';

export const api = axios.create({
  baseURL: 'http://localhost:/api/v',
  headers: { 'Content-Type': 'application/json' },
});

// Injection automatique du Token
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('auth_token');
  if (token) {
    config.headers.Authorization = Bearer ${token};
  }
  return config;
});

// Gestion automatique de l'expiration ()
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === ) {
      localStorage.removeItem('auth_token');
      window.location.href = '/login'; // Redirection forc√e
    }
    return Promise.reject(error);
  }
);