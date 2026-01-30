/ @type {import('tailwindcss').Config} /
export default {
  content: [
    "./index.html",
    "./src//.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class', 
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'sans-serif'], 
      },
      colors: {
        background: 'b', // Fond très sombre - Midnight Blue
        surface: 'b',    // Carte - Deep Navy
        border: 'a',     // Bordure subtile
        
        // Accents
        primary: 'bf',    // OpenDefender Blue
        
        // Semantic Risks
        risk: {
          low: 'b',      // Emerald
          medium: 'feb',   // Amber
          high: 'f',     // Orange
          critical: 'ef', // Red
        }
      },
      animation: {
        'fade-in': 'fadeIn .s ease-out',
        'glow-pulse': 'glowPulse s ease-in-out infinite',
        'neon-glow': 'neonGlow s ease-in-out infinite',
      },
      keyframes: {
        fadeIn: {
          '%': { opacity: '', transform: 'translateY(px)' },
          '%': { opacity: '', transform: 'translateY()' },
        },
        glowPulse: {
          '%, %': { boxShadow: '  px rgba(, , , .)' },
          '%': { boxShadow: '  px rgba(, , , .)' },
        },
        neonGlow: {
          '%, %': { 
            textShadow: '  px rgba(, , , .),   px rgba(, , , .)' 
          },
          '%': { 
            textShadow: '  px rgba(, , , .),   px rgba(, , , .)' 
          },
        }
      },
      backdropBlur: {
        'xl': 'px',
        'xl': 'px',
      },
      boxShadow: {
        'glow': '  px rgba(, , , .)',
        'glow-lg': '  px rgba(, , , .)',
        'glow-red': '  px rgba(, , , .)',
        'glow-orange': '  px rgba(, , , .)',
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
      }
    },
  },
  plugins: [],
}