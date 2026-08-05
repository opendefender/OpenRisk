/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  // Theme is driven by [data-theme] on <html> + CSS variables, so the `dark:`
  // variant is not used; tokens below resolve to the active theme automatically.
  darkMode: ['selector', '[data-theme="dark"]'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
        display: ['var(--font-display)', 'Inter', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'monospace'],
      },
      colors: {
        // ==== semantic tokens (src/styles/tokens.css) ====
        // These are the utilities components should use. Every one resolves to
        // a CSS variable, so a component written against them follows the theme
        // with no per-component work — which is the whole point: a raw palette
        // class like bg-zinc-900 is a constant and can never do that.
        'surface-0': 'var(--surface-0)',
        'surface-1': 'var(--surface-1)',
        'surface-2': 'var(--surface-2)',
        'surface-3': 'var(--surface-3)',
        'surface-sunken': 'var(--surface-sunken)',
        'surface-overlay': 'var(--surface-overlay)',
        'text-primary': 'var(--text-primary)',
        'text-secondary': 'var(--text-secondary)',
        'text-muted': 'var(--text-muted)',
        'text-inverse': 'var(--text-inverse)',
        'border-subtle': 'var(--border-subtle)',
        'border-default': 'var(--border-default)',
        success: {
          DEFAULT: 'var(--success)',
          surface: 'var(--success-surface)',
          text: 'var(--success-text)',
        },
        warning: {
          DEFAULT: 'var(--warning)',
          surface: 'var(--warning-surface)',
          text: 'var(--warning-text)',
        },
        danger: {
          DEFAULT: 'var(--danger)',
          surface: 'var(--danger-surface)',
          text: 'var(--danger-text)',
        },

        // ---- surfaces (CSS-var backed, swap with theme) ----
        app: 'var(--bg-app)',
        panel: 'var(--bg-primary)',
        elevated: 'var(--bg-elevated)',
        hover: 'var(--bg-hover)',
        glass: 'var(--glass)',
        'glass-strong': 'var(--glass-strong)',
        'glass-border': 'var(--glass-border)',
        // ---- legacy aliases kept so existing utility classes retint cleanly ----
        background: 'var(--bg-app)',
        surface: 'var(--bg-elevated)',
        border: 'var(--border)',
        'border-strong': 'var(--border-strong)',
        // ---- accent ----
        primary: 'var(--accent)', // legacy alias
        accent: {
          DEFAULT: 'var(--accent)',
          hover: 'var(--accent-hover)',
          2: 'var(--accent-2)',
          soft: 'var(--accent-soft)',
          line: 'var(--accent-line)',
          glow: 'var(--accent-glow)',
        },
        // ---- text ----
        ink: {
          DEFAULT: 'var(--text-primary)',
          soft: 'var(--text-secondary)',
          muted: 'var(--text-muted)',
        },
        // ---- semantic risk / status ----
        critical: 'var(--critical)',
        high: 'var(--high)',
        medium: 'var(--medium)',
        low: 'var(--low)',
        info: {
          DEFAULT: 'var(--info)',
          surface: 'var(--info-surface)',
          text: 'var(--info-text)',
        },
        risk: {
          low: 'var(--risk-low)',
          moderate: 'var(--risk-moderate)',
          medium: 'var(--risk-moderate)', // alias, existing usage
          high: 'var(--risk-high)',
          critical: 'var(--risk-critical)',
          extreme: 'var(--risk-extreme)',
        },
      },
      borderColor: {
        DEFAULT: 'var(--border-default)',
        subtle: 'var(--border-subtle)',
        strong: 'var(--border-strong)',
      },
      borderRadius: {
        token: 'var(--radius-md)',
        'token-lg': 'var(--radius-lg)',
        'token-xl': 'var(--radius-xl)',
      },
      boxShadow: {
        'card-sm': 'var(--shadow-sm)',
        'card-md': 'var(--shadow-md)',
        'card-lg': 'var(--shadow-lg)',
        overlay: 'var(--shadow-overlay)',
        glow: '0 3px 12px var(--accent-glow)',
        'glow-lg': '0 8px 28px var(--accent-glow)',
        'glow-red': '0 0 20px rgba(255, 69, 58, 0.5)',
        'glow-orange': '0 0 20px rgba(255, 159, 10, 0.5)',
      },
      animation: {
        'fade-in': 'fadeIn 0.5s ease-out',
        'glow-pulse': 'glowPulse 3s ease-in-out infinite',
        'neon-glow': 'neonGlow 2s ease-in-out infinite',
        // dc.html motion vocabulary (keyframes live in index.css)
        'or-fadeup': 'or-fadeup .4s ease both',
        'or-fadein': 'or-fadein .3s ease both',
        'or-scalein': 'or-scalein .18s cubic-bezier(.2,.8,.2,1) both',
        'or-slidein': 'or-slidein .3s cubic-bezier(.2,.8,.2,1) both',
        'or-pulsedot': 'or-pulsedot 1.5s infinite',
        'or-float': 'or-float 5s ease-in-out infinite',
        'or-shimmer': 'or-shimmer 1.4s infinite linear',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        glowPulse: {
          '0%, 100%': { boxShadow: '0 0 20px var(--accent-glow)' },
          '50%': { boxShadow: '0 0 40px var(--accent-glow)' },
        },
        neonGlow: {
          '0%, 100%': { textShadow: '0 0 10px var(--accent-glow)' },
          '50%': { textShadow: '0 0 20px var(--accent-glow)' },
        },
      },
      backdropBlur: {
        xl: '20px',
        '2xl': '40px',
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
      },
    },
  },
  plugins: [],
};
