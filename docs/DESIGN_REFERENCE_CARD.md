# 🎨 OpenRisk Dashboard Design - Visual Reference Card

## Color Palette Quick Reference

### Primary Colors
```
┌─────────────────────────────────────────┐
│ Background      #09090b (Deep Black)    │ ████
│ Surface         #18181b (Dark Navy)     │ ████
│ Border          #27272a (Subtle Gray)   │ ████
│ Primary         #3b82f6 (Bright Blue)   │ ████ ← Main accent
└─────────────────────────────────────────┘
```

### Risk Severity Colors
```
┌─────────────────────────────────────────┐
│ Critical        #ef4444 (Red)           │ ████ 🔴
│ High            #f97316 (Orange)        │ ████ 🟠
│ Medium          #eab308 (Yellow)        │ ████ 🟡
│ Low             #3b82f6 (Blue)          │ ████ 🔵
└─────────────────────────────────────────┘
```

### Supporting Colors
```
┌─────────────────────────────────────────┐
│ Success         #10b981 (Emerald)       │ ████ ✅
│ Warning         #f59e0b (Amber)         │ ████ ⚠️
│ Neutral         #71717a (Zinc)          │ ████ ⚪
└─────────────────────────────────────────┘
```

---

## Widget Layout Grid

```
12-Column Responsive Grid (Row Height: 80px)

┌──────────────────────────────────────────────────────────────┐
│                          HEADER                              │
│ Welcome back! | [Inventory] [Reset] [Export]                │
└──────────────────────────────────────────────────────────────┘

┌─────────────────────────┬─────────────────────────────────────┐
│  Risk Distribution      │  Risk Score Trends                  │
│  (6 cols × 4 rows)      │  (6 cols × 4 rows)                 │
│  - Donut Chart          │  - Line Chart                       │
│  - 4 segments           │  - 30 day history                   │
│  - Legend               │  - Glowing dots                     │
│  - Summary Stats        │  - Smooth animations                │
└─────────────────────────┴─────────────────────────────────────┘

┌─────────────────────────┬─────────────────────────────────────┐
│  Top Vulnerabilities    │  Avg Mitigation Time                │
│  (6 cols × 4 rows)      │  (6 cols × 4 rows)                 │
│  - Ranked list          │  - Semi-donut gauge                │
│  - Severity badges      │  - Time display                     │
│  - CVSS scores          │  - Completion stats                 │
│  - Asset counts         │  - Progress bar                     │
└─────────────────────────┴─────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────┐
│                    Key Indicators                            │
│  (12 cols × 3 rows)                                          │
│  ┌──────────────┬──────────────┬──────────────┬──────────────┐
│  │ Critical     │ Total        │ Mitigated    │ Total        │
│  │ Risks        │ Risks        │ Risks        │ Assets       │
│  │ ▲ 3          │ ▲ 50         │ ▲ 28/50      │ ▲ 145        │
│  └──────────────┴──────────────┴──────────────┴──────────────┘
└──────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────┐
│              Top Unmitigated Risks                           │
│  (12 cols × 4 rows)                                          │
│  1. 🔴 Critical Vuln...              SCORE: 18 →            │
│  2. 🟠 High Priority...              SCORE: 14 →            │
│  3. 🟠 Service Issue...              SCORE: 13 →            │
│  4. 🟡 Weak Control...               SCORE: 9  →            │
│  5. 🟡 Encryption Gap...             SCORE: 8  →            │
└──────────────────────────────────────────────────────────────┘
```

---

## Typography Reference

```
Page Title
├─ Font: Inter, 32px, Bold
├─ Color: White (#ffffff)
├─ Gradient: from-white to-blue-200
└─ Glow: text-shadow with 2s animation

Widget Title
├─ Font: Inter, 18px, Semibold
├─ Color: White (#ffffff)
├─ Icon: Lucide icon (20px) in primary blue
└─ Handle: Drag handle (16px gray icon)

Stat Label
├─ Font: Inter, 12px, Regular
├─ Color: Zinc-400 (#a1a1aa)
├─ Transform: Uppercase
└─ Tracking: Wider (0.05em)

Stat Value
├─ Font: Inter, 20px, Bold
├─ Color: White (#ffffff)
└─ Style: Monospace for numbers

Badge Text
├─ Font: Inter, 12px, Bold
├─ Color: Severity-based
├─ Background: Semi-transparent color
└─ Border: Matching color (0.3 opacity)

Tooltip Text
├─ Font: Inter, 11px, Regular
├─ Color: Zinc-900 (#18181b)
├─ Background: Dark blue (#18181b)
└─ Border: Subtle gray (#27272a)
```

---

## Component Sizing Guide

### Widget Cards
```
Desktop (1024px+):
├─ Half-width (6 cols): 48% - 24px margin
├─ Full-width (12 cols): 100% - 48px padding
├─ Height: 4 rows = 320px + 24px margin
├─ Border radius: 16px (rounded-2xl)
└─ Shadows: shadow-2xl with glow

Tablet (768px):
├─ Half-width stacks to full width
├─ Cards resize to available space
├─ Minimum width: 300px
└─ Touch-friendly padding: 16px

Mobile (360px):
├─ Full-width cards
├─ Padding: 16px sides
├─ Height: Auto or 300px
└─ List items: 2-3 visible before scroll
```

### Icon Sizing
```
Widget Title Icon:     20px (primary blue)
Drag Handle:          16px (zinc-600)
Severity Icons:       18px (color-coded)
Stat Icons:           18px (in cards)
Badge Icons:          14px (in badges)
Chevron Icons:        16px (subtle)
```

### Spacing System
```
Base Unit: 4px (Tailwind scale)

Padding:
  sm: 4px (p-1)
  md: 8px (p-2)
  lg: 16px (p-4)
  xl: 24px (p-6)

Gaps:
  sm: 4px (gap-1)
  md: 8px (gap-2)
  lg: 12px (gap-3)
  xl: 16px (gap-4)

Margins:
  Widget margin: 24px (between cards)
  Item margin: 12px (between list items)
```

---

## Animation Reference

### Fade In
```
Duration: 0.5s
Easing: ease-out
From: opacity-0, translateY(10px)
To: opacity-1, translateY(0)
```

### Glow Pulse
```
Duration: 3s
Easing: ease-in-out
Infinite: Yes
Effect: Box shadow pulsing
Colors: blue (5% to 80% opacity)
```

### Neon Glow (Text)
```
Duration: 2s
Easing: ease-in-out
Infinite: Yes
Effect: Text shadow flickering
Colors: blue text shadow
```

### Hover Scale
```
Duration: 200ms
Easing: ease
From: scale-100
To: scale-102 (2% increase)
```

### Grid Transitions
```
Duration: 200ms
Easing: ease
Properties: left, top, width, height
```

---

## Glassmorphism Effect Breakdown

```
Step 1: Background
  Linear Gradient: 135deg
  From: rgba(255, 255, 255, 0.05) 0%
  To: rgba(255, 255, 255, 0) 100%

Step 2: Backdrop
  Filter: blur(20px)
  -webkit-Filter: blur(20px) (Safari)

Step 3: Border
  Color: rgba(255, 255, 255, 0.1)
  Width: 1px
  Radius: 16px

Step 4: Shadow
  Box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3)

Step 5: Hover Effect
  Background: rgba(255, 255, 255, 0.08) 0%, rgba(255, 255, 255, 0.02) 100%
  Border: rgba(59, 130, 246, 0.3) (blue tint)
  Shadow: 0 8px 40px rgba(59, 130, 246, 0.2) (blue glow)
  Duration: 300ms smooth transition
```

---

## Neon Glow Reference

### Box Glow
```
Primary (Blue):
  box-shadow: 0 0 20px rgba(59, 130, 246, 0.5)

Critical (Red):
  box-shadow: 0 0 20px rgba(239, 68, 68, 0.5)

High (Orange):
  box-shadow: 0 0 20px rgba(249, 115, 22, 0.5)

Large Glow:
  box-shadow: 0 0 40px rgba(59, 130, 246, 0.5)
```

### Text Glow
```
Base Effect:
  text-shadow: 0 0 10px rgba(59, 130, 246, 0.5),
               0 0 20px rgba(59, 130, 246, 0.3)

Animated Effect (50% in animation):
  text-shadow: 0 0 20px rgba(59, 130, 246, 0.8),
               0 0 40px rgba(59, 130, 246, 0.5)
```

---

## Responsive Breakpoints

```
Mobile Small (360px)
├─ Single column layout
├─ Cards: 100% width - 32px padding
├─ Lists: 2-3 items visible
└─ Stat cards: 2×2 grid

Mobile Medium (480px)
├─ Single to dual column
├─ Cards: 100% width
├─ Improved spacing
└─ Better touch targets

Tablet (768px)
├─ 2 columns
├─ Side-by-side widgets
├─ Horizontal lists
└─ Stat cards: 4×1 or 2×2

Desktop (1024px+)
├─ Full 12-column grid
├─ Drag-and-drop enabled
├─ Side navigation visible
└─ All features accessible
```

---

## State Variations

### Widget States

**Default State:**
```
Border: rgba(255, 255, 255, 0.1)
Background: linear-gradient(from-white/5 to-white/0)
Shadow: shadow-2xl
```

**Hover State:**
```
Border: rgba(59, 130, 246, 0.3) ← Blue tint
Background: linear-gradient(from-white/8 to-white/2) ← Brighter
Shadow: shadow-2xl with blue glow
Duration: 300ms transition
```

**Dragging State:**
```
Opacity: 0.5 (semi-transparent)
Border: Same
Shadow: Lighter
Cursor: grabbing
```

**Loading State:**
```
Content: Spinner animation
Overlay: Semi-transparent
Message: "Loading..."
```

### Badge States

**Default:**
```
Background: Severity-based color (20% opacity)
Border: Severity-based color (30% opacity)
Text: Severity-based color (100% opacity)
```

**Hover (on parent):**
```
Background: Brighter (30% opacity)
Border: Brighter (40% opacity)
Glow: Severity-based box-shadow
Scale: 102%
```

---

## Accessibility Checklist

```
Color Contrast:
✅ Text on background: 7:1+ ratio (AAA)
✅ Icons on background: 4.5:1+ ratio (AA)
✅ Badges readable: High contrast maintained

Focus States:
✅ All interactive elements: Visible focus ring
✅ Keyboard navigation: Tab order logical
✅ Focus outline: 2px solid primary blue

Labels & Text:
✅ Icon + text labels: Combined clarity
✅ ARIA labels: On interactive elements
✅ Semantic HTML: Proper heading hierarchy

Motion:
✅ Reduced motion: Respects prefers-reduced-motion
✅ Animation speed: Not too fast or jarring
✅ Flashing: No content flashes > 3 times/sec
```

---

## Quick Copy-Paste Classes

### Text Effects
```
gradient-text: bg-gradient-to-r from-white to-blue-200 bg-clip-text text-transparent
neon-glow: animate-neon-glow
glow-pulse: animate-glow-pulse
fade-in: animate-fade-in
```

### Widget Effects
```
widget-glass: rounded-2xl border border-white/10 bg-gradient-to-br from-white/5 to-white/0 backdrop-blur-xl shadow-2xl
hover-glass: hover:border-white/20 hover:bg-gradient-to-br hover:from-white/8 hover:to-white/2 hover:shadow-glow-lg transition-all duration-300
badge-glow: badge-glow-critical (for critical severity)
neon-glow: box-shadow: 0 0 20px rgba(59, 130, 246, 0.5)
```

### Responsive Helpers
```
mobile-only: sm:hidden
desktop-only: hidden md:block
full-width: w-full
half-width: w-1/2 md:w-full
```

---

**Version**: 1.0  
**Last Updated**: January 2, 2026  
**Status**: ✅ Complete Reference
