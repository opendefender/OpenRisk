/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class', 
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'sans-serif'], 
      },
      colors: {
        background: '#09090b', // Fond très sombre - Midnight Blue
        surface: '#18181b',    // Carte - Deep Navy
        border: '#27272a',     // Bordure subtile
        
        // Accents
        primary: '#3b82f6',    // OpenDefender Blue
        
        // Semantic Risks
        risk: {
          low: '#10b981',      // Emerald
          medium: '#f59e0b',   // Amber
          high: '#f97316',     // Orange
          critical: '#ef4444', // Red
        }
      },
      animation: {
        'fade-in': 'fadeIn 0.5s ease-out',
        'glow-pulse': 'glowPulse 3s ease-in-out infinite',
        'neon-glow': 'neonGlow 2s ease-in-out infinite',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        glowPulse: {
          '0%, 100%': { boxShadow: '0 0 20px rgba(59, 130, 246, 0.5)' },
          '50%': { boxShadow: '0 0 40px rgba(59, 130, 246, 0.8)' },
        },
        neonGlow: {
          '0%, 100%': { 
            textShadow: '0 0 10px rgba(59, 130, 246, 0.5), 0 0 20px rgba(59, 130, 246, 0.3)' 
          },
          '50%': { 
            textShadow: '0 0 20px rgba(59, 130, 246, 0.8), 0 0 40px rgba(59, 130, 246, 0.5)' 
          },
        }
      },
      backdropBlur: {
        'xl': '20px',
        '2xl': '40px',
      },
      boxShadow: {
        'glow': '0 0 20px rgba(59, 130, 246, 0.5)',
        'glow-lg': '0 0 40px rgba(59, 130, 246, 0.5)',
        'glow-red': '0 0 20px rgba(239, 68, 68, 0.5)',
        'glow-orange': '0 0 20px rgba(249, 115, 22, 0.5)',
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
      }
    },
  },
  plugins: [],
}