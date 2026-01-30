 OpenRisk Dashboard - File Structure & Code Documentation

 Project Structure


frontend/src/
 features/
    dashboard/
        components/
           DashboardGrid.tsx (Main dashboard component -  lines)
           RiskDistribution.tsx (NEW - Donut chart widget)
           TopVulnerabilities.tsx (NEW - Vulnerability list widget)
           AverageMitigationTime.tsx (NEW - Gauge/progress widget)
           RiskTrendChart.tsx (Enhanced - Line chart widget)
           RiskMatrix.tsx (Legacy - kept for backward compatibility)
        widgets/
            GlobalScore.tsx
            RiskHeatmap.tsx
 components/
    layout/
       Sidebar.tsx
    ui/
        Button.tsx
 hooks/
    useRiskStore.ts
    useAssetStore.ts
    useAuthStore.ts
 types/
    react-grid-layout.d.ts (NEW - TypeScript definitions)
 App.tsx (Root component)
 App.css (Enhanced with glassmorphism & animations)
 index.css (Tailwind directives + global styles)
 main.tsx (Entry point)
 vite.config.ts (Build configuration)

tailwind.config.js (Enhanced with custom animations & colors)
package.json (Dependencies)
tsconfig.json (TypeScript configuration)


---

 Core Files & Their Responsibilities

 . DashboardGrid.tsx (Main Dashboard Component)
Purpose: Orchestrates the entire dashboard layout and widget arrangement

Key Features:
- -column responsive grid layout (react-grid-layout)
- Widget drag-and-drop functionality
- localStorage persistence for custom layouts
- Data fetching from multiple API endpoints
- Loading states and error handling
- Welcome header with user greeting
- Action buttons (Export, Reset, Inventory navigation)

Main Components Used:
tsx
import { RiskDistribution } from './RiskDistribution';
import { RiskTrendChart } from './RiskTrendChart';
import { TopVulnerabilities } from './TopVulnerabilities';
import { AverageMitigationTime } from './AverageMitigationTime';


Grid Layout:
typescript
const defaultLayout: Layout[] = [
  { i: 'risk-distribution', x: , y: , w: , h:  },
  { i: 'risk-trend', x: , y: , w: , h:  },
  { i: 'top-vulnerabilities', x: , y: , w: , h:  },
  { i: 'mitigation-time', x: , y: , w: , h:  },
  { i: 'key-indicators', x: , y: , w: , h:  },
  { i: 'top-risks', x: , y: , w: , h:  },
];


State Management:
tsx
const [layout, setLayout] = useState<Layout[]>(defaultLayout);
const [containerWidth, setContainerWidth] = useState();
const { risks, fetchRisks, isLoading } = useRiskStore();
const { assets, fetchAssets, isLoading } = useAssetStore();
const { user } = useAuthStore();


Custom Components:
tsx
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


---

 . RiskDistribution.tsx (Donut Chart Widget)
Purpose: Display risk breakdown by severity level

API Endpoint: /stats/risk-distribution

Data Structure:
typescript
interface RiskDistributionData {
  critical: number;
  high: number;
  medium: number;
  low: number;
}

const chartData = [
  { name: 'Critical', value: data.critical, color: 'ef' },
  { name: 'High', value: data.high, color: 'f' },
  { name: 'Medium', value: data.medium, color: 'eab' },
  { name: 'Low', value: data.low, color: 'bf' },
];


Chart Configuration:
tsx
<PieChart>
  <Pie
    data={chartData}
    cx="%"
    cy="%"
    innerRadius={}          // Makes it a donut
    outerRadius={}
    paddingAngle={}          // Space between segments
    dataKey="value"
  >
    {chartData.map(entry => (
      <Cell fill={entry.color} />
    ))}
  </Pie>
  <Tooltip ... />
</PieChart>


Features:
- Interactive donut chart with hover tooltips
- Color-coded by severity
- Legend showing count and label
- Summary statistics card
- Fallback demo data
- Loading state with spinner

---

 . RiskTrendChart.tsx (Line Chart Widget)
Purpose: Display risk score trends over  days

API Endpoint: /stats/trends

Data Structure:
typescript
interface TrendPoint {
  date: string;  // Format: "--"
  score: number; // Range: -
}


Chart Configuration:
tsx
<LineChart data={data}>
  <Line
    type="monotone"
    dataKey="score"
    stroke="bf"           // Primary blue
    strokeWidth={}
    dot={{                      // Animated glowing dots
      fill: 'bf',
      r: ,
      filter: 'url(glow)'
    }}
    activeDot={{ r: , fill: 'afa' }}
  />
</LineChart>


Features:
- Smooth animated line with glowing effect
- Interactive cursor and tooltip
- Y-axis range - for standard scoring
- X-axis shows day numbers
- Grid lines for reference
- Trend indicator (positive if score decreasing)
- Fallback demo data with -day history

---

 . TopVulnerabilities.tsx (Vulnerability List Widget)
Purpose: Ranked list of top security vulnerabilities

API Endpoint: /stats/top-vulnerabilities

Data Structure:
typescript
interface Vulnerability {
  id: string;
  title: string;
  severity: 'Critical' | 'High' | 'Medium' | 'Low';
  cvssScore?: number;
  affectedAssets?: number;
}


Severity Color Mapping:
typescript
const getSeverityColor = (severity: string) => {
  switch (severity.toLowerCase()) {
    case 'critical':
      return { 
        bg: 'bg-red-/', 
        text: 'text-red-', 
        border: 'border-red-/',
        badge: 'bg-red-/' 
      };
    case 'high':
      return {
        bg: 'bg-orange-/',
        text: 'text-orange-',
        border: 'border-orange-/',
        badge: 'bg-orange-/'
      };
    // ... medium and low
  }
};


Fallback Demo Data:
typescript
[
  {
    id: '',
    title: 'SQL Injection',
    severity: 'Critical',
    cvssScore: .,
    affectedAssets: ,
  },
  {
    id: '',
    title: 'Cross-Site Scripting (XSS)',
    severity: 'High',
    cvssScore: .,
    affectedAssets: ,
  },
  // ...
]


Features:
- Ranked by severity/score
- Severity icons (octagon/triangle/circle)
- Color-coded badges
- CVSS score display
- Affected asset count
- Scrollable list
- Hover effects with scale animation
- Drill-down link support

---

 . AverageMitigationTime.tsx (Gauge Widget)
Purpose: Display mitigation metrics and completion rate

API Endpoint: /stats/mitigation-metrics

Data Structure:
typescript
interface MitigationMetrics {
  averageTimeHours: number;      // Total hours
  averageTimeDays: number;        // Converted days
  completedCount: number;         // Number completed
  pendingCount: number;           // Number pending
  completionRate: number;         // - percentage
}


Gauge Chart:
tsx
<PieChart>
  <Pie
    data={[
      { name: 'Completed', value: completedCount, color: 'b' },
      { name: 'Pending', value: pendingCount, color: 'ef' },
    ]}
    cx="%"
    cy="%"
    innerRadius={}
    outerRadius={}
    startAngle={}              // Semi-circle (gauge)
    endAngle={}
    paddingAngle={}
  />
</PieChart>


Display Format:
- Center: Time in format "Xh Ym" (hours and minutes)
- Stats Cards: Completed/Pending counts with color backgrounds
- Progress Bar: Visual completion percentage with gradient
- All values fetched from API or fallback demo data

Features:
- Semi-donut gauge visualization
- Center display of average time
- Color-coded stat cards
- Animated progress bar
- Responsive metric display

---

 Styling System

 Tailwind Configuration (tailwind.config.js)

Custom Colors:
javascript
colors: {
  background: 'b',    // Deep black (RGB: , , )
  surface: 'b',       // Dark blue-gray (RGB: , , )
  border: 'a',        // Subtle border (RGB: , , )
  primary: 'bf',       // Bright blue
  risk: {
    low: 'b',         // Emerald
    medium: 'feb',      // Amber
    high: 'f',        // Orange
    critical: 'ef',    // Red
  }
}


Custom Animations:
javascript
animation: {
  'glow-pulse': 'glowPulse s ease-in-out infinite',
  'neon-glow': 'neonGlow s ease-in-out infinite',
  'fade-in': 'fadeIn .s ease-out',
}

keyframes: {
  glowPulse: {
    '%, %': { boxShadow: '  px rgba(, , , .)' },
    '%': { boxShadow: '  px rgba(, , , .)' },
  },
  neonGlow: {
    '%, %': { textShadow: '  px rgba(, , , .), ...' },
    '%': { textShadow: '  px rgba(, , , .), ...' },
  }
}


Custom Utilities:
javascript
backdropBlur: {
  'xl': 'px',
  'xl': 'px',
}

boxShadow: {
  'glow': '  px rgba(, , , .)',
  'glow-lg': '  px rgba(, , , .)',
  'glow-red': '  px rgba(, , , .)',
  'glow-orange': '  px rgba(, , , .)',
}


 CSS Classes (App.css)

Glassmorphic Widget:
css
.widget-glass {
  border-radius: rem;
  border: px solid rgba(, , , .);
  background: linear-gradient(deg, rgba(, , , .) %, rgba(, , , ) %);
  backdrop-filter: blur(px);
  box-shadow:  px px rgba(, , , .);
}

.widget-glass:hover {
  border-color: rgba(, , , .);
  background: linear-gradient(deg, rgba(, , , .) %, rgba(, , , .) %);
  box-shadow:  px px rgba(, , , .);
}


Neon Glow Effect:
css
.neon-glow {
  box-shadow:   px rgba(, , , .);
  animation: neonFlicker s ease-in-out infinite;
}

@keyframes neonFlicker {
  %, % { box-shadow:   px rgba(, , , .); }
  % { box-shadow:   px rgba(, , , .); }
}


---

 Component Props & Interfaces

 GlassmorphicWidget
typescript
interface WidgetProps {
  title: string;                // Widget title
  children: React.ReactNode;    // Widget content
  className?: string;           // Additional CSS classes
  padding?: string;             // Padding override (default: 'p-')
  isDragging?: boolean;         // Dragging state
  icon?: React.ElementType;     // Optional icon (Lucide)
}


 StatCard
typescript
interface StatCardProps {
  label: string;                      // Stat label
  value: string | number;             // Stat value
  icon: React.ElementType;            // Icon component
  color?: string;                     // Icon color class
}


---

 API Integration Points

 Data Fetching Pattern
typescript
useEffect(() => {
  api.get('/stats/risk-distribution')
    .then(res => setData(res.data || getDemoData()))
    .catch(() => setData(getDemoData()))  // Fallback
    .finally(() => setIsLoading(false));
}, []);


 API Endpoints
| Endpoint | Purpose | Data Type |
|----------|---------|-----------|
| /stats/risk-distribution | Risk counts by severity | { critical, high, medium, low } |
| /stats/trends | -day risk score history | [{ date, score }, ...] |
| /stats/top-vulnerabilities | Top security issues | [{ id, title, severity, cvssScore }, ...] |
| /stats/mitigation-metrics | Mitigation performance | { averageTimeHours, completedCount, completionRate, ... } |
| /stats/risk-matrix | Risk matrix data | Matrix cell data (legacy) |
| /export/pdf | PDF report generation | Binary PDF file |

---

 Performance Optimizations

 . Lazy Loading
typescript
// Components load data independently
useEffect(() => {
  fetchData();
}, []); // Only on mount


 . Data Memoization
typescript
const chartData = useMemo(() => {
  return risks.map(risk => ({
    date: risk.createdAt,
    score: risk.score
  }));
}, [risks]);


 . Hardware Acceleration
- CSS transforms for animations (GPU-accelerated)
- will-change hints for expensive operations
- transform: translated() for smooth transitions

 . Efficient Rendering
- React hooks prevent unnecessary re-renders
- Component isolation prevents cascade updates
- GridLayout optimized for resize operations

---

 Testing & Debugging

 Fallback Data (Development)
typescript
const getDemoData = (): RiskDistributionData => ({
  critical: ,
  high: ,
  medium: ,
  low: 
});


All components include fallback demo data for:
- Testing without API
- Development/staging
- Error scenarios
- Visual regression testing

 Console Logging (if needed)
typescript
console.error("Failed to fetch trends data:", error);
// Falls back to demo data silently


---

 Browser Compatibility

- Chrome/Edge: Full support (modern CSS, backdrop-filter)
- Firefox: Full support
- Safari: Full support (with -webkit prefixes)
- Mobile Browsers: Responsive design tested

CSS Fallbacks:
- Backdrop blur has solid color fallback
- Gradients use fallback colors
- Animations gracefully degrade

---

 Accessibility Features

. Semantic HTML: Proper heading hierarchy, semantic elements
. Color Contrast: All text meets WCAG AA standard
. Icon Labels: Icons accompanied by text labels
. Keyboard Navigation: Full keyboard support for interactive elements
. ARIA: Proper aria-labels and roles where needed
. Focus States: Visible focus indicators on interactive elements
. Readable Fonts: Inter at px minimum with proper line-height

---

 Future Extension Points

 Adding a New Widget
typescript
// . Create new component in /dashboard/components/
// . Add to DashboardGrid imports
// . Add to defaultLayout configuration
// . Add grid item in render section

<div key="new-widget">
  <GlassmorphicWidget title="New Widget" icon={SomeIcon}>
    <YourComponent />
  </GlassmorphicWidget>
</div>


 Adding New Animations
javascript
// In tailwind.config.js
animation: {
  'your-animation': 'yourKeyframe s ease infinite',
},
keyframes: {
  yourKeyframe: {
    '%': { / start / },
    '%': { / end / },
  }
}


---

Documentation Version: .  
Last Updated: January ,   
Status:  Complete
