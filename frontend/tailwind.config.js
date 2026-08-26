/** @type {import('tailwindcss').Config} */

/*
 * Tailwind is the delivery mechanism for the design tokens, not a second
 * design system. Every value below resolves to a CSS variable defined in
 * src/styles/primitives.css (scales) or src/styles/tokens.css (colour), so a
 * utility written in a component follows the theme with no per-component work.
 *
 * The rule this file encodes: if a visual value is worth using twice, it is a
 * token and it gets a utility here. An arbitrary value in a component
 * (text-[13px], rounded-[10px], z-[70]) means either the token is missing or
 * the component is inventing one — both are bugs. See
 * docs/W1-01_OPENRISK_DESIGN_SYSTEM.md.
 */
export default {
  /* e2e is scanned too. The visual harness and the design-system gallery are
     the only places that render some variants (a disabled destructive button,
     an outlined `experimental` badge), and Tailwind only emits a utility it has
     seen used. Without this the gallery renders those states with the class
     absent from the stylesheet — a snapshot suite quietly photographing
     unstyled markup, which is worse than no suite. */
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}', './e2e/**/*.{ts,tsx}'],
  // Theme is driven by [data-theme] on <html> + CSS variables, so the `dark:`
  // variant is not used; tokens below resolve to the active theme automatically.
  darkMode: ['selector', '[data-theme="dark"]'],
  theme: {
    // Mirrors --bp-* in primitives.css. 3xl is the master-detail threshold.
    screens: {
      sm: '640px',
      md: '768px',
      lg: '1024px',
      xl: '1280px',
      '2xl': '1536px',
      '3xl': '1920px',
    },
    extend: {
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
        display: ['var(--font-display)', 'Inter', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'monospace'],
      },

      /* Type scale. Each step carries its line height, so `text-sm` is a
         complete typographic decision rather than half of one — the missing
         half is why 1559 arbitrary `text-[13px]` calls existed, each of them
         also silently inheriting whatever leading was nearby. */
      fontSize: {
        '2xs': ['var(--text-2xs)', { lineHeight: 'var(--leading-snug)' }],
        xs: ['var(--text-xs)', { lineHeight: 'var(--leading-snug)' }],
        sm: ['var(--text-sm)', { lineHeight: 'var(--leading-normal)' }],
        base: ['var(--text-base)', { lineHeight: 'var(--leading-normal)' }],
        md: ['var(--text-md)', { lineHeight: 'var(--leading-snug)' }],
        lg: ['var(--text-lg)', { lineHeight: 'var(--leading-snug)' }],
        xl: ['var(--text-xl)', { lineHeight: 'var(--leading-tight)' }],
        '2xl': ['var(--text-2xl)', { lineHeight: 'var(--leading-tight)' }],
        '3xl': ['var(--text-3xl)', { lineHeight: 'var(--leading-tight)' }],
      },
      letterSpacing: {
        display: 'var(--tracking-display)',
        caps: 'var(--tracking-caps)',
      },
      lineHeight: {
        none: 'var(--leading-none)',
        tight: 'var(--leading-tight)',
        snug: 'var(--leading-snug)',
        normal: 'var(--leading-normal)',
        relaxed: 'var(--leading-relaxed)',
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
        'border-control': 'var(--border-control)',
        focus: 'var(--focus-ring-color)',
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
          solid: 'var(--danger-solid)',
        },
        'success-solid': 'var(--success-solid)',
        'warning-solid': 'var(--warning-solid)',
        'info-solid': 'var(--info-solid)',
        'accent-solid': 'var(--accent-solid)',
        'text-on-solid': 'var(--text-on-solid)',

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
        // ---- visualisation: one categorical palette for every chart ----
        chart: {
          1: 'var(--chart-1)',
          2: 'var(--chart-2)',
          3: 'var(--chart-3)',
          4: 'var(--chart-4)',
          5: 'var(--chart-5)',
          6: 'var(--chart-6)',
          7: 'var(--chart-7)',
          8: 'var(--chart-8)',
          grid: 'var(--chart-grid)',
          axis: 'var(--chart-axis)',
          label: 'var(--chart-label)',
          track: 'var(--chart-track)',
        },
        graph: {
          node: 'var(--graph-node)',
          stroke: 'var(--graph-node-stroke)',
          edge: 'var(--graph-edge)',
          active: 'var(--graph-edge-active)',
        },
      },
      borderColor: {
        DEFAULT: 'var(--border-default)',
        subtle: 'var(--border-subtle)',
        strong: 'var(--border-strong)',
        control: 'var(--border-control)',
      },
      borderRadius: {
        xs: 'var(--radius-xs)',
        sm: 'var(--radius-sm)',
        md: 'var(--radius-md)',
        lg: 'var(--radius-lg)',
        xl: 'var(--radius-xl)',
        // Legacy aliases from before the scale was named; identical values.
        token: 'var(--radius-md)',
        'token-lg': 'var(--radius-lg)',
        'token-xl': 'var(--radius-xl)',
      },

      /* Elevation. Four steps, theme-aware, and no decorative variants: the
         `glow`/`neon` shadows that used to live here were removed in W1-01 —
         a glow on the primary button is what made the product read as a
         template rather than an instrument. */
      boxShadow: {
        'elev-1': 'var(--elev-1)',
        'elev-2': 'var(--elev-2)',
        'elev-3': 'var(--elev-3)',
        overlay: 'var(--elev-overlay)',
        // Legacy aliases; same four values.
        'card-sm': 'var(--elev-1)',
        'card-md': 'var(--elev-2)',
        'card-lg': 'var(--elev-3)',
      },

      /* Named stacking layers. Replaces the 10 arbitrary integers that were in
         use (59, 60, 65, 70, 75, 80, 85, 90, 95, 120). The order is the
         contract — note that tooltip outranks modal, because a tooltip on a
         control inside a dialog has to be readable. */
      zIndex: {
        base: 'var(--z-base)',
        raised: 'var(--z-raised)',
        sticky: 'var(--z-sticky)',
        nav: 'var(--z-nav)',
        header: 'var(--z-header)',
        dropdown: 'var(--z-dropdown)',
        drawer: 'var(--z-drawer)',
        modal: 'var(--z-modal)',
        popover: 'var(--z-popover)',
        toast: 'var(--z-toast)',
        tooltip: 'var(--z-tooltip)',
      },

      transitionDuration: {
        instant: 'var(--dur-instant)',
        fast: 'var(--dur-fast)',
        base: 'var(--dur-base)',
        slow: 'var(--dur-slow)',
        panel: 'var(--dur-panel)',
      },
      transitionTimingFunction: {
        out: 'var(--ease-out)',
        in: 'var(--ease-in)',
        inout: 'var(--ease-inout)',
        emphasized: 'var(--ease-emphasized)',
      },

      animation: {
        'fade-in': 'or-fadein var(--dur-slow) var(--ease-out) both',
        // The OpenRisk motion vocabulary (keyframes live in index.css).
        'or-fadeup': 'or-fadeup var(--dur-slow) var(--ease-out) both',
        'or-fadein': 'or-fadein var(--dur-base) var(--ease-out) both',
        'or-scalein': 'or-scalein var(--dur-base) var(--ease-out) both',
        'or-rise': 'or-rise var(--dur-base) var(--ease-out) both',
        'or-slidein': 'or-slidein var(--dur-panel) var(--ease-out) both',
        'or-pulsedot': 'or-pulsedot 1.5s infinite',
        'or-shimmer': 'or-shimmer 1.4s infinite linear',
      },
      backdropBlur: {
        xl: '20px',
        '2xl': '40px',
      },
    },
  },
  plugins: [],
};
