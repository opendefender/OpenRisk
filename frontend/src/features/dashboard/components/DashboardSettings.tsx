import { useEffect, useState } from 'react';
import { Save, RotateCcw, Eye, EyeOff, Settings } from 'lucide-react';

interface DashboardWidget {
  id: string;
  name: string;
  visible: boolean;
  enabled: boolean;
}

interface DashboardConfig {
  widgets: DashboardWidget[];
  refreshInterval: number; // in seconds
  theme: 'dark' | 'light';
  compactMode: boolean;
  autoRefresh: boolean;
}

interface DashboardSettingsProps {
  onConfigChange?: (config: DashboardConfig) => void;
  onClose?: () => void;
}

const DEFAULT_WIDGETS: DashboardWidget[] = [
  { id: 'risk-distribution', name: 'Risk Distribution', visible: true, enabled: true },
  { id: 'risk-trend', name: 'Risk Trends (30/60/90d)', visible: true, enabled: true },
  { id: 'top-vulnerabilities', name: 'Top Vulnerabilities', visible: true, enabled: true },
  { id: 'mitigation-time', name: 'Average Mitigation Time', visible: true, enabled: true },
  { id: 'security-score', name: 'Security Score', visible: true, enabled: true },
  { id: 'asset-statistics', name: 'Asset Statistics', visible: true, enabled: true },
  { id: 'framework-analytics', name: 'Framework Compliance', visible: true, enabled: true },
  { id: 'key-indicators', name: 'Key Indicators', visible: true, enabled: true },
  { id: 'top-risks', name: 'Top Unmitigated Risks', visible: true, enabled: true },
  { id: 'risk-matrix', name: 'Risk Matrix (Heatmap)', visible: true, enabled: true },
];

const STORAGE_KEY = 'openrisk-dashboard-config';

/**
 * DashboardSettings Component
 * Allows users to customize dashboard: show/hide widgets, set refresh interval, etc.
 * Persists preferences to localStorage
 */
export const DashboardSettings: React.FC<DashboardSettingsProps> = ({
  onConfigChange,
  onClose,
}) => {
  const [config, setConfig] = useState<DashboardConfig>(() => {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) {
      return JSON.parse(stored);
    }
    return {
      widgets: DEFAULT_WIDGETS,
      refreshInterval: 30,
      theme: 'dark',
      compactMode: false,
      autoRefresh: true,
    };
  });

  const [isDirty, setIsDirty] = useState(false);

  // Handle widget visibility toggle
  const toggleWidgetVisibility = (widgetId: string) => {
    setConfig((prev) => ({
      ...prev,
      widgets: prev.widgets.map((w) =>
        w.id === widgetId ? { ...w, visible: !w.visible } : w
      ),
    }));
    setIsDirty(true);
  };

  // Handle widget enable/disable
  const toggleWidgetEnabled = (widgetId: string) => {
    setConfig((prev) => ({
      ...prev,
      widgets: prev.widgets.map((w) =>
        w.id === widgetId ? { ...w, enabled: !w.enabled } : w
      ),
    }));
    setIsDirty(true);
  };

  // Handle refresh interval change
  const updateRefreshInterval = (value: number) => {
    setConfig((prev) => ({ ...prev, refreshInterval: value }));
    setIsDirty(true);
  };

  // Handle auto-refresh toggle
  const toggleAutoRefresh = () => {
    setConfig((prev) => ({ ...prev, autoRefresh: !prev.autoRefresh }));
    setIsDirty(true);
  };

  // Handle theme change
  const updateTheme = (theme: 'dark' | 'light') => {
    setConfig((prev) => ({ ...prev, theme }));
    setIsDirty(true);
  };

  // Handle compact mode toggle
  const toggleCompactMode = () => {
    setConfig((prev) => ({ ...prev, compactMode: !prev.compactMode }));
    setIsDirty(true);
  };

  // Save configuration
  const handleSave = () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(config));
    setIsDirty(false);
    if (onConfigChange) {
      onConfigChange(config);
    }
  };

  // Reset to defaults
  const handleReset = () => {
    const defaultConfig: DashboardConfig = {
      widgets: DEFAULT_WIDGETS,
      refreshInterval: 30,
      theme: 'dark',
      compactMode: false,
      autoRefresh: true,
    };
    setConfig(defaultConfig);
    setIsDirty(true);
  };

  return (
    <div className="w-full max-w-2xl mx-auto">
      {/* Header */}
      <div className="flex items-center justify-between mb-6 pb-4 border-b border-white/10">
        <div className="flex items-center gap-2">
          <Settings size={20} className="text-primary" />
          <h2 className="text-lg font-semibold text-white">Dashboard Settings</h2>
        </div>
        {onClose && (
          <button
            onClick={onClose}
            className="text-zinc-400 hover:text-white transition-colors"
          >
            ✕
          </button>
        )}
      </div>

      <div className="space-y-6">
        {/* General Settings */}
        <div>
          <h3 className="text-sm font-semibold text-zinc-300 mb-3 uppercase tracking-wider">General Settings</h3>
          <div className="space-y-3">
            {/* Theme */}
            <div className="flex items-center justify-between p-3 rounded-lg bg-white/5 border border-white/10">
              <label className="text-sm text-zinc-300">Theme</label>
              <div className="flex gap-2">
                {(['dark', 'light'] as const).map((theme) => (
                  <button
                    key={theme}
                    onClick={() => updateTheme(theme)}
                    className={`px-3 py-1 rounded text-xs font-semibold transition-all ${
                      config.theme === theme
                        ? 'bg-primary text-white'
                        : 'bg-white/10 text-zinc-400 hover:bg-white/20'
                    }`}
                  >
                    {theme.charAt(0).toUpperCase() + theme.slice(1)}
                  </button>
                ))}
              </div>
            </div>

            {/* Auto-Refresh */}
            <div className="flex items-center justify-between p-3 rounded-lg bg-white/5 border border-white/10">
              <label className="text-sm text-zinc-300">Auto-Refresh Data</label>
              <button
                onClick={toggleAutoRefresh}
                className={`relative w-10 h-6 rounded-full transition-colors ${
                  config.autoRefresh ? 'bg-primary' : 'bg-zinc-600'
                }`}
              >
                <div
                  className={`absolute w-5 h-5 bg-white rounded-full top-0.5 transition-transform ${
                    config.autoRefresh ? 'translate-x-5' : 'translate-x-0.5'
                  }`}
                />
              </button>
            </div>

            {/* Refresh Interval */}
            {config.autoRefresh && (
              <div className="flex items-center justify-between p-3 rounded-lg bg-white/5 border border-white/10">
                <label className="text-sm text-zinc-300">Refresh Interval (seconds)</label>
                <div className="flex items-center gap-2">
                  <input
                    type="range"
                    min="10"
                    max="300"
                    step="10"
                    value={config.refreshInterval}
                    onChange={(e) => updateRefreshInterval(parseInt(e.target.value))}
                    className="w-32"
                  />
                  <span className="text-sm font-semibold text-primary min-w-[40px]">
                    {config.refreshInterval}s
                  </span>
                </div>
              </div>
            )}

            {/* Compact Mode */}
            <div className="flex items-center justify-between p-3 rounded-lg bg-white/5 border border-white/10">
              <label className="text-sm text-zinc-300">Compact Mode</label>
              <button
                onClick={toggleCompactMode}
                className={`relative w-10 h-6 rounded-full transition-colors ${
                  config.compactMode ? 'bg-primary' : 'bg-zinc-600'
                }`}
              >
                <div
                  className={`absolute w-5 h-5 bg-white rounded-full top-0.5 transition-transform ${
                    config.compactMode ? 'translate-x-5' : 'translate-x-0.5'
                  }`}
                />
              </button>
            </div>
          </div>
        </div>

        {/* Widget Management */}
        <div>
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-sm font-semibold text-zinc-300 uppercase tracking-wider">Widget Management</h3>
            <span className="text-xs text-zinc-500">
              {config.widgets.filter((w) => w.visible).length} / {config.widgets.length} visible
            </span>
          </div>

          <div className="space-y-2 max-h-64 overflow-y-auto pr-2">
            {config.widgets.map((widget) => (
              <div
                key={widget.id}
                className="flex items-center justify-between p-3 rounded-lg bg-white/5 border border-white/10 hover:bg-white/10 transition-colors"
              >
                <div className="flex-1">
                  <p className="text-sm font-medium text-white">{widget.name}</p>
                  <p className="text-xs text-zinc-500">{widget.id}</p>
                </div>

                <div className="flex items-center gap-2 flex-shrink-0">
                  {/* Visibility Toggle */}
                  <button
                    onClick={() => toggleWidgetVisibility(widget.id)}
                    className="p-2 rounded-lg hover:bg-white/10 transition-colors text-zinc-400 hover:text-white"
                    title={widget.visible ? 'Hide widget' : 'Show widget'}
                  >
                    {widget.visible ? (
                      <Eye size={16} />
                    ) : (
                      <EyeOff size={16} />
                    )}
                  </button>

                  {/* Enable/Disable Toggle */}
                  <button
                    onClick={() => toggleWidgetEnabled(widget.id)}
                    className={`relative w-10 h-6 rounded-full transition-colors ${
                      widget.enabled ? 'bg-primary' : 'bg-zinc-600'
                    }`}
                    title={widget.enabled ? 'Disable widget' : 'Enable widget'}
                  >
                    <div
                      className={`absolute w-5 h-5 bg-white rounded-full top-0.5 transition-transform ${
                        widget.enabled ? 'translate-x-5' : 'translate-x-0.5'
                      }`}
                    />
                  </button>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Action Buttons */}
        <div className="flex gap-3 pt-4 border-t border-white/10">
          <button
            onClick={handleReset}
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-white/5 hover:bg-white/10 text-zinc-400 hover:text-white transition-colors text-sm font-semibold border border-white/10"
          >
            <RotateCcw size={16} />
            Reset to Defaults
          </button>

          <div className="flex-1" />

          {onClose && (
            <button
              onClick={onClose}
              className="px-4 py-2 rounded-lg bg-white/5 hover:bg-white/10 text-zinc-400 hover:text-white transition-colors text-sm font-semibold border border-white/10"
            >
              Close
            </button>
          )}

          <button
            onClick={handleSave}
            disabled={!isDirty}
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-primary hover:bg-primary/90 disabled:bg-zinc-600 disabled:cursor-not-allowed text-white transition-colors text-sm font-semibold"
          >
            <Save size={16} />
            Save Settings
          </button>
        </div>
      </div>

      {/* Info Box */}
      <div className="mt-6 p-3 rounded-lg bg-blue-500/10 border border-blue-500/30 text-xs text-blue-300">
        💡 Your dashboard preferences are saved locally in your browser. Widget visibility and settings will persist across sessions.
      </div>
    </div>
  );
};

/**
 * Export configuration loader for DashboardGrid
 * Use this to load the saved configuration
 */
export const loadDashboardConfig = (): DashboardConfig => {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored) {
    return JSON.parse(stored);
  }
  return {
    widgets: DEFAULT_WIDGETS,
    refreshInterval: 30,
    theme: 'dark',
    compactMode: false,
    autoRefresh: true,
  };
};

/**
 * Export configuration saver
 */
export const saveDashboardConfig = (config: DashboardConfig): void => {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(config));
};

/**
 * Export default configuration getter
 */
export const getDefaultDashboardConfig = (): DashboardConfig => ({
  widgets: DEFAULT_WIDGETS,
  refreshInterval: 30,
  theme: 'dark',
  compactMode: false,
  autoRefresh: true,
});
