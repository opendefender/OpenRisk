# OpenRisk Dashboard - File Structure & Code Documentation

## Project Structure

```
frontend/src/
├── features/
│   └── dashboard/
│       ├── components/
│       │   ├── DashboardGrid.tsx (Main dashboard component - 280 lines)
│       │   ├── RiskDistribution.tsx (NEW - Donut chart widget)
│       │   ├── TopVulnerabilities.tsx (NEW - Vulnerability list widget)
│       │   ├── AverageMitigationTime.tsx (NEW - Gauge/progress widget)
│       │   ├── RiskTrendChart.tsx (Enhanced - Line chart widget)
│       │   └── RiskMatrix.tsx (Legacy - kept for backward compatibility)
│       └── widgets/
│           ├── GlobalScore.tsx
│           └── RiskHeatmap.tsx
├── components/
│   ├── layout/
│   │   └── Sidebar.tsx
│   └── ui/
│       └── Button.tsx
├── hooks/
│   ├── useRiskStore.ts
│   ├── useAssetStore.ts
│   └── useAuthStore.ts
├── types/
│   └── react-grid-layout.d.ts (NEW - TypeScript definitions)
├── App.tsx (Root component)
├── App.css (Enhanced with glassmorphism & animations)
├── index.css (Tailwind directives + global styles)
├── main.tsx (Entry point)
└── vite.config.ts (Build configuration)

tailwind.config.js (Enhanced with custom animations & colors)
package.json (Dependencies)
tsconfig.json (TypeScript configuration)
```

---

## Core Files & Their Responsibilities

### 1. **DashboardGrid.tsx** (Main Dashboard Component)
**Purpose**: Orchestrates the entire dashboard layout and widget arrangement

**Key Features**:
- 12-column responsive grid layout (react-grid-layout)
- Widget drag-and-drop functionality
- localStorage persistence for custom layouts
- Data fetching from multiple API endpoints
- Loading states and error handling
- Welcome header with user greeting
- Action buttons (Export, Reset, Inventory navigation)

**Main Components Used**:
```tsx
import { RiskDistribution } from './RiskDistribution';
import { RiskTrendChart } from './RiskTrendChart';
import { TopVulnerabilities } from './TopVulnerabilities';
import { AverageMitigationTime } from './AverageMitigationTime';
```

**Grid Layout**:
```typescript
const defaultLayout: Layout[] = [
  { i: 'risk-distribution', x: 0, y: 0, w: 6, h: 4 },
  { i: 'risk-trend', x: 6, y: 0, w: 6, h: 4 },
  { i: 'top-vulnerabilities', x: 0, y: 4, w: 6, h: 4 },
  { i: 'mitigation-time', x: 6, y: 4, w: 6, h: 4 },
  { i: 'key-indicators', x: 0, y: 8, w: 12, h: 3 },
  { i: 'top-risks', x: 0, y: 11, w: 12, h: 4 },
];
```

**State Management**:
```tsx
const [layout, setLayout] = useState<Layout[]>(defaultLayout);
const [containerWidth, setContainerWidth] = useState(1200);
const { risks, fetchRisks, isLoading } = useRiskStore();
const { assets, fetchAssets, isLoading } = useAssetStore();
const { user } = useAuthStore();
```

**Custom Components**:
```tsx
interface GlassmorphicWidget {
  title: string;
  children: React.ReactNode;
  icon?: React.ElementType;
  className?: string;
  padding?: string;
  isDragging?: boolean;
}

const GlassmorphicWidget: React.FC<WidgetProps> = ({ ... })
const StatCard: React.FC<StatCardProps> = ({ ... })
```

---

### 2. **RiskDistribution.tsx** (Donut Chart Widget)
**Purpose**: Display risk breakdown by severity level

**API Endpoint**: `/stats/risk-distribution`

**Data Structure**:
```typescript
interface RiskDistributionData {
  critical: number;
  high: number;
  medium: number;
  low: number;
}

const chartData = [
  { name: 'Critical', value: data.critical, color: '#ef4444' },
  { name: 'High', value: data.high, color: '#f97316' },
  { name: 'Medium', value: data.medium, color: '#eab308' },
  { name: 'Low', value: data.low, color: '#3b82f6' },
];
```

**Chart Configuration**:
```tsx
<PieChart>
  <Pie
    data={chartData}
    cx="50%"
    cy="50%"
    innerRadius={60}          // Makes it a donut
    outerRadius={100}
    paddingAngle={2}          // Space between segments
    dataKey="value"
  >
    {chartData.map(entry => (
      <Cell fill={entry.color} />
    ))}
  </Pie>
  <Tooltip ... />
</PieChart>
```

**Features**:
- Interactive donut chart with hover tooltips
- Color-coded by severity
- Legend showing count and label
- Summary statistics card
- Fallback demo data
- Loading state with spinner

---

### 3. **RiskTrendChart.tsx** (Line Chart Widget)
**Purpose**: Display risk score trends over 30 days

**API Endpoint**: `/stats/trends`

**Data Structure**:
```typescript
interface TrendPoint {
  date: string;  // Format: "2024-12-01"
  score: number; // Range: 0-100
}
```

**Chart Configuration**:
```tsx
<LineChart data={data}>
  <Line
    type="monotone"
    dataKey="score"
    stroke="#3b82f6"           // Primary blue
    strokeWidth={3}
    dot={{                      // Animated glowing dots
      fill: '#3b82f6',
      r: 4,
      filter: 'url(#glow)'
    }}
    activeDot={{ r: 6, fill: '#60a5fa' }}
  />
</LineChart>
```

**Features**:
- Smooth animated line with glowing effect
- Interactive cursor and tooltip
- Y-axis range 0-100 for standard scoring
- X-axis shows day numbers
- Grid lines for reference
- Trend indicator (positive if score decreasing)
- Fallback demo data with 7-day history

---

### 4. **TopVulnerabilities.tsx** (Vulnerability List Widget)
**Purpose**: Ranked list of top security vulnerabilities

**API Endpoint**: `/stats/top-vulnerabilities`

**Data Structure**:
```typescript
interface Vulnerability {
  id: string;
  title: string;
  severity: 'Critical' | 'High' | 'Medium' | 'Low';
  cvssScore?: number;
  affectedAssets?: number;
}
```

**Severity Color Mapping**:
```typescript
const getSeverityColor = (severity: string) => {
  switch (severity.toLowerCase()) {
    case 'critical':
      return { 
        bg: 'bg-red-500/10', 
        text: 'text-red-400', 
        border: 'border-red-500/30',
        badge: 'bg-red-500/20' 
      };
    case 'high':
      return {
        bg: 'bg-orange-500/10',
        text: 'text-orange-400',
        border: 'border-orange-500/30',
        badge: 'bg-orange-500/20'
      };
    // ... medium and low
  }
};
```

**Fallback Demo Data**:
```typescript
[
  {
    id: '1',
    title: 'SQL Injection',
    severity: 'Critical',
    cvssScore: 9.8,
    affectedAssets: 3,
  },
  {
    id: '2',
    title: 'Cross-Site Scripting (XSS)',
    severity: 'High',
    cvssScore: 7.5,
    affectedAssets: 5,
  },
  // ...
]
```

**Features**:
- Ranked by severity/score
- Severity icons (octagon/triangle/circle)
- Color-coded badges
- CVSS score display
- Affected asset count
- Scrollable list
- Hover effects with scale animation
- Drill-down link support

---

### 5. **AverageMitigationTime.tsx** (Gauge Widget)
**Purpose**: Display mitigation metrics and completion rate

**API Endpoint**: `/stats/mitigation-metrics`

**Data Structure**:
```typescript
interface MitigationMetrics {
  averageTimeHours: number;      // Total hours
  averageTimeDays: number;        // Converted days
  completedCount: number;         // Number completed
  pendingCount: number;           // Number pending
  completionRate: number;         // 0-100 percentage
}
```

**Gauge Chart**:
```tsx
<PieChart>
  <Pie
    data={[
      { name: 'Completed', value: completedCount, color: '#10b981' },
      { name: 'Pending', value: pendingCount, color: '#ef4444' },
    ]}
    cx="50%"
    cy="50%"
    innerRadius={50}
    outerRadius={75}
    startAngle={180}              // Semi-circle (gauge)
    endAngle={0}
    paddingAngle={2}
  />
</PieChart>
```

**Display Format**:
- Center: Time in format "Xh Ym" (hours and minutes)
- Stats Cards: Completed/Pending counts with color backgrounds
- Progress Bar: Visual completion percentage with gradient
- All values fetched from API or fallback demo data

**Features**:
- Semi-donut gauge visualization
- Center display of average time
- Color-coded stat cards
- Animated progress bar
- Responsive metric display

---

## Styling System

### Tailwind Configuration (`tailwind.config.js`)

**Custom Colors**:
```javascript
colors: {
  background: '#09090b',    // Deep black (RGB: 9, 9, 11)
  surface: '#18181b',       // Dark blue-gray (RGB: 24, 24, 27)
  border: '#27272a',        // Subtle border (RGB: 39, 39, 42)
  primary: '#3b82f6',       // Bright blue
  risk: {
    low: '#10b981',         // Emerald
    medium: '#f59e0b',      // Amber
    high: '#f97316',        // Orange
    critical: '#ef4444',    // Red
  }
}
```

**Custom Animations**:
```javascript
animation: {
  'glow-pulse': 'glowPulse 3s ease-in-out infinite',
  'neon-glow': 'neonGlow 2s ease-in-out infinite',
  'fade-in': 'fadeIn 0.5s ease-out',
}

keyframes: {
  glowPulse: {
    '0%, 100%': { boxShadow: '0 0 20px rgba(59, 130, 246, 0.5)' },
    '50%': { boxShadow: '0 0 40px rgba(59, 130, 246, 0.8)' },
  },
  neonGlow: {
    '0%, 100%': { textShadow: '0 0 10px rgba(59, 130, 246, 0.5), ...' },
    '50%': { textShadow: '0 0 20px rgba(59, 130, 246, 0.8), ...' },
  }
}
```

**Custom Utilities**:
```javascript
backdropBlur: {
  'xl': '20px',
  '2xl': '40px',
}

boxShadow: {
  'glow': '0 0 20px rgba(59, 130, 246, 0.5)',
  'glow-lg': '0 0 40px rgba(59, 130, 246, 0.5)',
  'glow-red': '0 0 20px rgba(239, 68, 68, 0.5)',
  'glow-orange': '0 0 20px rgba(249, 115, 22, 0.5)',
}
```

### CSS Classes (`App.css`)

**Glassmorphic Widget**:
```css
.widget-glass {
  border-radius: 1rem;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.05) 0%, rgba(255, 255, 255, 0) 100%);
  backdrop-filter: blur(20px);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
}

.widget-glass:hover {
  border-color: rgba(59, 130, 246, 0.3);
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.08) 0%, rgba(255, 255, 255, 0.02) 100%);
  box-shadow: 0 8px 40px rgba(59, 130, 246, 0.2);
}
```

**Neon Glow Effect**:
```css
.neon-glow {
  box-shadow: 0 0 20px rgba(59, 130, 246, 0.5);
  animation: neonFlicker 3s ease-in-out infinite;
}

@keyframes neonFlicker {
  0%, 100% { box-shadow: 0 0 20px rgba(59, 130, 246, 0.5); }
  50% { box-shadow: 0 0 40px rgba(59, 130, 246, 0.8); }
}
```

---

## Component Props & Interfaces

### GlassmorphicWidget
```typescript
interface WidgetProps {
  title: string;                // Widget title
  children: React.ReactNode;    // Widget content
  className?: string;           // Additional CSS classes
  padding?: string;             // Padding override (default: 'p-6')
  isDragging?: boolean;         // Dragging state
  icon?: React.ElementType;     // Optional icon (Lucide)
}
```

### StatCard
```typescript
interface StatCardProps {
  label: string;                      // Stat label
  value: string | number;             // Stat value
  icon: React.ElementType;            // Icon component
  color?: string;                     // Icon color class
}
```

---

## API Integration Points

### Data Fetching Pattern
```typescript
useEffect(() => {
  api.get('/stats/risk-distribution')
    .then(res => setData(res.data || getDemoData()))
    .catch(() => setData(getDemoData()))  // Fallback
    .finally(() => setIsLoading(false));
}, []);
```

### API Endpoints
| Endpoint | Purpose | Data Type |
|----------|---------|-----------|
| `/stats/risk-distribution` | Risk counts by severity | `{ critical, high, medium, low }` |
| `/stats/trends` | 30-day risk score history | `[{ date, score }, ...]` |
| `/stats/top-vulnerabilities` | Top security issues | `[{ id, title, severity, cvssScore }, ...]` |
| `/stats/mitigation-metrics` | Mitigation performance | `{ averageTimeHours, completedCount, completionRate, ... }` |
| `/stats/risk-matrix` | Risk matrix data | Matrix cell data (legacy) |
| `/export/pdf` | PDF report generation | Binary PDF file |

---

## Performance Optimizations

### 1. **Lazy Loading**
```typescript
// Components load data independently
useEffect(() => {
  fetchData();
}, []); // Only on mount
```

### 2. **Data Memoization**
```typescript
const chartData = useMemo(() => {
  return risks.map(risk => ({
    date: risk.createdAt,
    score: risk.score
  }));
}, [risks]);
```

### 3. **Hardware Acceleration**
- CSS transforms for animations (GPU-accelerated)
- `will-change` hints for expensive operations
- `transform: translate3d()` for smooth transitions

### 4. **Efficient Rendering**
- React hooks prevent unnecessary re-renders
- Component isolation prevents cascade updates
- GridLayout optimized for resize operations

---

## Testing & Debugging

### Fallback Data (Development)
```typescript
const getDemoData = (): RiskDistributionData => ({
  critical: 3,
  high: 8,
  medium: 15,
  low: 24
});
```

All components include fallback demo data for:
- Testing without API
- Development/staging
- Error scenarios
- Visual regression testing

### Console Logging (if needed)
```typescript
console.error("Failed to fetch trends data:", error);
// Falls back to demo data silently
```

---

## Browser Compatibility

- **Chrome/Edge**: Full support (modern CSS, backdrop-filter)
- **Firefox**: Full support
- **Safari**: Full support (with -webkit prefixes)
- **Mobile Browsers**: Responsive design tested

**CSS Fallbacks**:
- Backdrop blur has solid color fallback
- Gradients use fallback colors
- Animations gracefully degrade

---

## Accessibility Features

1. **Semantic HTML**: Proper heading hierarchy, semantic elements
2. **Color Contrast**: All text meets WCAG AA standard
3. **Icon Labels**: Icons accompanied by text labels
4. **Keyboard Navigation**: Full keyboard support for interactive elements
5. **ARIA**: Proper aria-labels and roles where needed
6. **Focus States**: Visible focus indicators on interactive elements
7. **Readable Fonts**: Inter at 14px minimum with proper line-height

---

## Future Extension Points

### Adding a New Widget
```typescript
// 1. Create new component in /dashboard/components/
// 2. Add to DashboardGrid imports
// 3. Add to defaultLayout configuration
// 4. Add grid item in render section

<div key="new-widget">
  <GlassmorphicWidget title="New Widget" icon={SomeIcon}>
    <YourComponent />
  </GlassmorphicWidget>
</div>
```

### Adding New Animations
```javascript
// In tailwind.config.js
animation: {
  'your-animation': 'yourKeyframe 2s ease infinite',
},
keyframes: {
  yourKeyframe: {
    '0%': { /* start */ },
    '100%': { /* end */ },
  }
}
```

---

**Documentation Version**: 1.0  
**Last Updated**: January 2, 2026  
**Status**: ✅ Complete
