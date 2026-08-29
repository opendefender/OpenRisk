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
      // At 390px the header carries a logo, a language switch, a menu and the
      // primary action. This is the stop where something has to give.
      xs: '400px',
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

        /* The display scale. Fluid, so no heading needs a breakpoint of its
           own, and the line height and tracking are baked in for the same
           reason the console steps are: a size is half a typographic decision
           and the other half is what goes missing. Used by the surfaces that
           are read rather than scanned — sign-in, empty states, upgrade. */
        eyebrow: ['0.6875rem', { lineHeight: '1rem', letterSpacing: 'var(--tracking-eyebrow)' }],
        'display-1': ['var(--display-1)', { lineHeight: '1.02', letterSpacing: '-0.034em' }],
        'display-2': ['var(--display-2)', { lineHeight: '1.08', letterSpacing: '-0.028em' }],
        'display-3': ['var(--display-3)', { lineHeight: '1.16', letterSpacing: '-0.022em' }],
        'display-4': ['var(--display-4)', { lineHeight: '1.25', letterSpacing: '-0.015em' }],
        lead: ['var(--lead)', { lineHeight: '1.55', letterSpacing: '-0.008em' }],
      },
      letterSpacing: {
        display: 'var(--tracking-display)',
        caps: 'var(--tracking-caps)',
        eyebrow: 'var(--tracking-eyebrow)',
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
        // The accent step that is safe as small TEXT in both themes. `accent` is
        // the MARK — a keyline, an active border, a fill — and is held to 3:1;
        // this one carries the 4.5:1 that a label needs. `text-accent` already
        // resolves here (see textColor below), so reach for this name when the
        // accent has to colour something that is not text but sits next to it.
        'accent-strong': 'var(--accent-500)',
        // Hairlines and the measured field. Both neutral: a grid is a
        // coordinate space, not an atmosphere.
        hairline: 'var(--hairline)',
        'grid-line': 'var(--grid-line)',
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
      /* `text-accent` resolves to the TEXT step, `bg-accent` and `border-accent`
         to the MARK. One utility name cannot fork by property, so the fork is
         declared here — which is what lets the signature ultramarine stay the
         signature ultramarine on a rule while every accent LABEL in the product
         still clears 4.5:1 on every surface it can land on. The rest of the
         accent keys are repeated verbatim; a bare string here would replace the
         whole nested object and silently delete `text-accent-hover`. */
      textColor: {
        accent: {
          DEFAULT: 'var(--accent-500)',
          strong: 'var(--accent-500)',
          hover: 'var(--accent-hover)',
          2: 'var(--accent-2)',
          soft: 'var(--accent-soft)',
          line: 'var(--accent-line)',
          glow: 'var(--accent-glow)',
        },
      },

      borderColor: {
        DEFAULT: 'var(--border-default)',
        subtle: 'var(--border-subtle)',
        strong: 'var(--border-strong)',
        control: 'var(--border-control)',
      },
      /* THE KEYLINE — the OpenRisk signature. A 2px accent rule marking the
         active element: the selected nav item, the current tab, the focused
         panel, the row being acted on. It replaces the filled-pill / glow /
         gradient vocabulary that makes every dashboard look like every other
         dashboard, and it costs one border instead of a background. Until now
         the token existed with no utility to spend it through. */
      borderWidth: {
        keyline: 'var(--keyline-w)',
      },

      maxWidth: {
        content: 'var(--content-max)',
        'content-wide': 'var(--content-max-wide)',
        prose: 'var(--prose-max)',
        shell: 'var(--shell-max)',
      },

      spacing: {
        section: 'var(--space-section)',
        'section-sm': 'var(--space-section-sm)',
        /* Density-aware: a row built from these responds to the density switch
           without the component knowing which density is active. */
        row: 'var(--den-row)',
        'cell-y': 'var(--den-cell-y)',
        control: 'var(--control-h-md)',
        header: 'var(--header-h)',
        sidebar: 'var(--sidebar-w)',
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
