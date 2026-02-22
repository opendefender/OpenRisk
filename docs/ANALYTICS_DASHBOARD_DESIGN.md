# Analytics Dashboard Layout Design

**Date**: February 22, 2026  
**Phase**: Phase 6 - Advanced Analytics & Monitoring  
**Status**: Design Complete

---

## Overview

The Real-Time Analytics Dashboard provides comprehensive visibility into OpenRisk metrics, trends, and operational insights. It features a modular, responsive design with customizable widgets and real-time data updates.

---

## Architecture & Layout

### Page Structure

```
┌─────────────────────────────────────────────────────────┐
│ Dashboard Toolbar                                       │
│ [Title] [Refresh] [Settings] [Filter] [Export]         │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌──────────┐  ┌──────────────────────────────────────┐ │
│  │ Filters  │  │ KPI Cards (3 columns)               │ │
│  │          │  │ ┌────────┐ ┌────────┐ ┌────────┐    │ │
│  │ • Period │  │ │Risks   │ │Progress│ │Coverage│    │ │
│  │ • Metrics│  │ │287     │ │68%     │ │92%     │    │ │
│  │ • Date   │  │ └────────┘ └────────┘ └────────┘    │ │
│  │          │  └──────────────────────────────────────┘ │
│  └──────────┘                                          │
│                                                         │
│  ┌─────────────────────────┐  ┌──────────────────────┐ │
│  │ Risk Trends Chart       │  │ Risk Severity Chart  │ │
│  │ (Area Chart)            │  │ (Pie Chart)          │ │
│  └─────────────────────────┘  └──────────────────────┘ │
│                                                         │
│  ┌─────────────────────────┐  ┌──────────────────────┐ │
│  │ Top Risks Table         │  │ Mitigation Progress  │ │
│  │                         │  │ Table                │ │
│  └─────────────────────────┘  └──────────────────────┘ │
│                                                         │
│  ┌─────────────────────────┐  ┌──────────────────────┐ │
│  │ Mitigation Status Chart │  │ Key Metrics Summary  │ │
│  │ (Bar Chart)             │  │                      │ │
│  └─────────────────────────┘  └──────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

---

## Components

### 1. **Toolbar** (`AnalyticsDashboardToolbar`)

**Purpose**: Provides dashboard controls and navigation

**Features**:
- Dashboard title and description
- Refresh button with loading indicator
- Settings button for customization
- Filter button for data filtering
- Export button for data export

**Props**:
```typescript
interface AnalyticsDashboardToolbarProps {
  onFilterClick?: () => void;
  onExportClick?: () => void;
  onRefreshClick?: () => void;
  onSettingsClick?: () => void;
  isLoading?: boolean;
}
```

### 2. **Metric Cards** (`MetricCard`)

**Purpose**: Display key performance indicators

**Features**:
- Large value display with unit
- Trend indicator (up/down/stable)
- Percentage change comparison
- Status indicator (normal/warning/critical)
- Optional click handler for drill-down

**Layout**:
```
┌──────────────────────────┐
│ Title        [Status]    │
│ Description              │
│                          │
│ 287          active      │
│ +12% vs last period      │
└──────────────────────────┘
```

**Props**:
```typescript
interface MetricCardProps {
  title: string;
  value: string | number;
  unit?: string;
  change?: number;
  changePercent?: number;
  isPositive?: boolean;
  trend?: 'up' | 'down' | 'stable';
  status?: 'normal' | 'warning' | 'critical';
  description?: string;
  onClick?: () => void;
}
```

### 3. **Chart Widget** (`ChartWidget`)

**Purpose**: Container for data visualizations

**Features**:
- Flexible height options (small/medium/large)
- Optional action button
- Scrollable content area
- Consistent styling with other components
- Support for full-width layout

**Height Options**:
- Small: 16rem (256px)
- Medium: 20rem (320px)
- Large: 24rem (384px)

**Props**:
```typescript
interface ChartWidgetProps {
  title: string;
  subtitle?: string;
  children: React.ReactNode;
  action?: {
    label: string;
    onClick: () => void;
  };
  fullWidth?: boolean;
  height?: 'small' | 'medium' | 'large';
}
```

### 4. **Dashboard Filter** (`DashboardFilter`)

**Purpose**: Allow users to customize displayed data

**Features**:
- Quick period selection (Today, 7d, 30d, 90d, YTD, Custom)
- Custom date range picker
- Multi-select metric filter
- Persistent state management

**Filter Options**:
| Label | Value | Use Case |
|-------|-------|----------|
| Today | today | Daily review |
| Last 7 Days | 7d | Weekly trends |
| Last 30 Days | 30d | Monthly review |
| Last 90 Days | 90d | Quarterly analysis |
| Year to Date | ytd | Annual comparison |
| Custom | custom | Specific date ranges |

**Props**:
```typescript
interface DashboardFilterProps {
  selectedPeriod: string;
  onPeriodChange: (period: string) => void;
  selectedMetrics?: string[];
  onMetricsChange?: (metrics: string[]) => void;
  dateRange?: {
    start: string;
    end: string;
  };
  onDateRangeChange?: (start: string, end: string) => void;
}
```

### 5. **Data Table Widget** (`DataTableWidget`)

**Purpose**: Display tabular data with sorting and actions

**Features**:
- Dynamic column definitions
- Status indicators (active/warning/critical)
- Trend visualization
- Row click handler for drill-down
- More actions menu
- Empty state handling

**Data Structure**:
```typescript
interface DataItem {
  id: string;
  name: string;
  value: number;
  trend?: number;
  status?: 'active' | 'warning' | 'critical';
}
```

**Props**:
```typescript
interface DataTableWidgetProps {
  title: string;
  columns: string[];
  data: DataItem[];
  onRowClick?: (item: DataItem) => void;
  onViewMore?: () => void;
}
```

---

## Data Visualizations

### 1. **Risk Trends** (Area Chart)
- **Data**: Daily risk counts over 7-day period
- **Metrics**: Total risks, mitigations, assets
- **Interaction**: Hover for details
- **Export**: PNG/CSV

### 2. **Risk Severity Distribution** (Pie Chart)
- **Data**: Risks by severity level
- **Categories**: Critical (Red), High (Orange), Medium (Yellow), Low (Green)
- **Display**: Count and percentage
- **Total**: 150 risks

### 3. **Mitigation Status** (Bar Chart)
- **Categories**: Completed, In Progress, Not Started, Overdue
- **Metrics**: Count per category
- **Colors**: Green, Blue, Gray, Red

### 4. **Top Risks** (Data Table)
- **Columns**: Risk Name, Score, Trend
- **Sorting**: By score (descending)
- **Actions**: Click for details, view mitigations

### 5. **Mitigation Progress** (Data Table)
- **Columns**: Mitigation, Progress %, Trend
- **Sorting**: By progress (descending)
- **Actions**: Click for timeline, update status

### 6. **Key Metrics Summary** (Stats Panel)
- Average Risk Score
- Risks Trending Up/Down
- Overdue Mitigations Count
- SLA Compliance %

---

## Responsive Design

### Breakpoints

| Screen Size | Layout |
|-------------|--------|
| Mobile (< 640px) | Single column, stacked cards |
| Tablet (640-1024px) | 2 columns for most sections |
| Desktop (> 1024px) | 3-4 columns with sidebar filter |

### Mobile Optimization

- Filter sidebar collapses to modal
- KPI cards stack vertically
- Charts maintain readability at smaller sizes
- Tables scroll horizontally
- Toolbar buttons wrap if needed

---

## Data Flow

### Real-Time Updates

1. **Polling**: Update every 30 seconds during active viewing
2. **WebSocket** (future): Live updates for critical metrics
3. **Refresh Button**: Manual trigger for immediate update
4. **Background Refresh**: Continue updates even when unfocused (reduced frequency)

### Caching Strategy

- **Short-lived cache** (5 minutes): KPI cards, trend data
- **Medium cache** (15 minutes): Risk severity distribution
- **Long-lived cache** (1 hour): Historical trends
- **No cache**: Real-time alerts and critical metrics

---

## Color Scheme

### Status Colors

```css
/* Normal - Blue */
--status-normal: #3B82F6 (Blue-500)

/* Warning - Yellow */
--status-warning: #FBBF24 (Amber-400)

/* Critical - Red */
--status-critical: #EF4444 (Red-500)

/* Success - Green */
--status-success: #22C55E (Green-500)
```

### Severity Colors (Risk)

```css
--severity-critical: #EF4444 (Red)
--severity-high: #F97316 (Orange)
--severity-medium: #FBBF24 (Yellow)
--severity-low: #22C55E (Green)
```

---

## Accessibility

### WCAG 2.1 Compliance

- ✅ Color not sole differentiator (symbols + colors)
- ✅ Keyboard navigation (Tab, Enter, Arrow keys)
- ✅ Screen reader support (ARIA labels)
- ✅ Focus indicators (Blue outline on hover)
- ✅ Sufficient contrast (4.5:1 minimum)
- ✅ Responsive text sizing

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| R | Refresh data |
| F | Toggle filter panel |
| E | Export data |
| ? | Show help |
| Esc | Close modals |

---

## Performance Optimization

### Rendering

- **Lazy loading**: Charts load on visibility
- **Virtual scrolling**: Large tables use windowing
- **Memoization**: Components prevent unnecessary re-renders
- **Code splitting**: Dashboard loaded separately

### Data Loading

- **Pagination**: Tables paginate by 50 rows
- **Aggregation**: Backend pre-computes trend data
- **Compression**: API responses gzipped
- **Delta updates**: Only changed metrics sent

### Bundle Impact

- Initial load: ~150KB (gzipped)
- Chart library: ~85KB (Recharts)
- Total page load: < 2s on 4G

---

## Future Enhancements

### Phase 6.2 - Advanced Features

1. **WebSocket Real-Time Updates**
   - Live metric streaming
   - Sub-second latency for critical alerts
   - Automatic reconnection with backoff

2. **Custom Dashboard Widgets**
   - Drag-and-drop layout editor
   - User-defined metric combinations
   - Save multiple dashboard layouts

3. **Advanced Filtering**
   - Filter by risk properties (owner, category, asset)
   - Saved filter presets
   - Filter history and suggestions

4. **Predictive Analytics**
   - Trend forecasting (30-day outlook)
   - Risk escalation predictions
   - Mitigation impact modeling

5. **Collaboration Features**
   - Dashboard sharing with teams
   - Annotations and comments
   - @mentions for alerts

### Phase 6.3 - ML Integration

1. **Anomaly Detection**
   - Unusual risk pattern detection
   - Alert on spike detection
   - Auto-investigation suggestions

2. **Recommendations**
   - Mitigation priority suggestions
   - Resource allocation optimization
   - Risk grouping recommendations

---

## Implementation Checklist

### Core Components
- [x] Toolbar component
- [x] Metric card component
- [x] Chart widget component
- [x] Dashboard filter component
- [x] Data table widget component
- [x] Main dashboard page

### Visualizations
- [x] Risk trends area chart
- [x] Risk severity pie chart
- [x] Mitigation status bar chart
- [x] Top risks data table
- [x] Mitigation progress table
- [x] Key metrics summary panel

### Features
- [ ] Real-time data updates
- [ ] Export functionality (CSV, PDF)
- [ ] Custom date range filtering
- [ ] Metric selection persistence
- [ ] Dashboard customization
- [ ] Print-friendly layout

### Testing
- [ ] Unit tests for components
- [ ] Integration tests for data flow
- [ ] E2E tests for user workflows
- [ ] Accessibility testing (WCAG 2.1)
- [ ] Performance testing (Lighthouse)
- [ ] Cross-browser testing

---

## File Structure

```
frontend/src/
├── components/
│   └── dashboard/
│       ├── AnalyticsDashboardToolbar.tsx
│       ├── MetricCard.tsx
│       ├── ChartWidget.tsx
│       ├── DashboardFilter.tsx
│       └── DataTableWidget.tsx
├── pages/
│   └── RealTimeAnalyticsDashboard.tsx
├── types/
│   └── analytics.ts (type definitions)
├── hooks/
│   └── useAnalyticsDashboard.ts (data fetching)
└── styles/
    └── dashboard.css (custom styles)
```

---

## Related Documentation

- [Phase 6 Strategic Roadmap](../../PHASE6_STRATEGIC_ROADMAP.md)
- [Design System Reference](../../docs/DESIGN_SYSTEM_COMPLETE_SUMMARY.md)
- [API Endpoints](../../docs/API_REFERENCE.md)

---

**Status**: Ready for Development  
**Next Phase**: Data Integration & Real-Time Updates  
**Estimated Completion**: March 15, 2026
