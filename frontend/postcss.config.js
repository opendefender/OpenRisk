/* Copyright (c) 2026 OpenDefender Contributors
   SPDX-License-Identifier: AGPL-3.0-only */

/* Tailwind v4 ships its own PostCSS plugin, which also does the @import
   inlining that postcss-import used to do and the vendor prefixing that
   autoprefixer used to do. Both are therefore gone from devDependencies. */
export default {
  plugins: {
    '@tailwindcss/postcss': {},
  },
}
