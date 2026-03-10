/**
 * Advanced Search/Typeahead Component
 * Provides intelligent risk search with fuzzy matching and keyboard navigation
 */

import React, { useRef } from 'react';
import { Search, Loader, Clock, AlertCircle, Zap } from 'lucide-react';
import { useTypeahead, useKeyboardShortcuts } from '@/hooks/useTypeahead';
import type { TypeaheadResult } from '@/hooks/useTypeahead';

interface AdvancedSearchProps {
  onSelect?: (result: TypeaheadResult) => void;
  placeholder?: string;
  showShortcutHint?: boolean;
}

export const AdvancedSearch: React.FC<AdvancedSearchProps> = ({
  onSelect,
  placeholder = 'Search risks... (Cmd+K)',
  showShortcutHint = true,
}) => {
  const inputRef = useRef<HTMLInputElement>(null);
  const {
    state,
    handleInput,
    handleKeyDown,
    containerRef,
    selectedItemRef,
    saveRecentSearch,
    clearRecentSearches,
  } = useTypeahead({
    minChars: 1,
    maxResults: 8,
    debounceMs: 200,
    enableFuzzyMatch: true,
    enableRecentSearches: true,
  });

  // Focus search on Cmd+K
  useKeyboardShortcuts(() => {
    inputRef.current?.focus();
  });

  const handleSelect = (result: TypeaheadResult) => {
    saveRecentSearch(result);
    onSelect?.(result);
    handleInput(''); // Clear input
  };

  const displayResults =
    state.query.length >= 2
      ? state.results
      : state.isOpen && state.recentSearches.length > 0
        ? state.recentSearches
        : [];

  return (
    <div ref={containerRef} className="relative w-full">
      <div className="relative">
        {/* Search Input */}
        <div className="relative flex items-center bg-zinc-900 border border-zinc-700 rounded-lg focus-within:border-blue-500 focus-within:ring-1 focus-within:ring-blue-500/20 transition-all">
          <Search
            className="absolute left-3 text-zinc-400 pointer-events-none"
            size={18}
          />
          <input
            ref={inputRef}
            type="text"
            placeholder={placeholder}
            value={state.query}
            onChange={(e) => handleInput(e.target.value)}
            onFocus={() => {
              if (state.query.length >= 2 || state.recentSearches.length > 0) {
                // @ts-ignore - setState in state
                state.isOpen = true;
              }
            }}
            onKeyDown={handleKeyDown}
            className="w-full bg-transparent pl-10 pr-10 py-2.5 text-white placeholder-zinc-400 outline-none"
            autoComplete="off"
          />

          {/* Right icons */}
          <div className="absolute right-3 flex items-center gap-2">
            {state.isLoading && (
              <Loader className="animate-spin text-blue-500" size={16} />
            )}
            {showShortcutHint && !state.query && (
              <kbd className="hidden md:flex items-center gap-1 px-2 py-1 bg-zinc-800 rounded text-xs text-zinc-400 border border-zinc-700">
                <span>⌘</span>
                <span>K</span>
              </kbd>
            )}
          </div>
        </div>

        {/* Dropdown Results */}
        {state.isOpen && (
          <div className="absolute top-full left-0 right-0 mt-2 bg-zinc-900 border border-zinc-700 rounded-lg shadow-xl z-50 overflow-hidden">
            {/* Results List */}
            {displayResults.length > 0 ? (
              <div className="max-h-96 overflow-y-auto">
                {state.query.length >= 2 && state.results.length > 0 && (
                  <div className="px-3 py-2 text-xs text-zinc-400 border-b border-zinc-800 flex items-center gap-2">
                    <Zap size={14} />
                    <span>Search Results ({state.results.length})</span>
                  </div>
                )}

                {state.results.map((result, index) => (
                  <button
                    key={result.id}
                    ref={index === state.selectedIndex ? selectedItemRef : null}
                    onClick={() => handleSelect(result)}
                    className={`w-full text-left px-4 py-3 border-b border-zinc-800 last:border-b-0 transition-colors ${
                      index === state.selectedIndex
                        ? 'bg-blue-500/20 border-l-2 border-l-blue-500'
                        : 'hover:bg-zinc-800'
                    }`}
                  >
                    <div className="flex items-start justify-between">
                      <div className="flex-1 min-w-0">
                        <h3 className="text-white font-medium truncate">
                          {result.title}
                        </h3>
                        {result.description && (
                          <p className="text-xs text-zinc-400 truncate mt-1">
                            {result.description}
                          </p>
                        )}
                      </div>

                      {/* Score Badge */}
                      {result.score !== undefined && (
                        <div className="ml-2 flex-shrink-0">
                          <div
                            className={`text-xs font-bold px-2 py-1 rounded ${
                              result.score >= 15
                                ? 'bg-red-500/20 text-red-400'
                                : result.score >= 9
                                  ? 'bg-yellow-500/20 text-yellow-400'
                                  : result.score >= 5
                                    ? 'bg-orange-500/20 text-orange-400'
                                    : 'bg-green-500/20 text-green-400'
                            }`}
                          >
                            {result.score.toFixed(1)}
                          </div>
                        </div>
                      )}
                    </div>

                    {/* Metadata Row */}
                    {result.probability !== undefined && result.impact !== undefined && (
                      <div className="flex items-center gap-4 mt-2 text-xs text-zinc-400">
                        <span>📊 P:{result.probability} I:{result.impact}</span>
                        {result.matchScore !== undefined && (
                          <span className="text-blue-400">
                            Match: {Math.round(result.matchScore * 100)}%
                          </span>
                        )}
                      </div>
                    )}
                  </button>
                ))}

                {/* Recent Searches Header */}
                {state.query.length < 2 && state.recentSearches.length > 0 && (
                  <div className="px-3 py-2 text-xs text-zinc-400 border-b border-zinc-800 flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Clock size={14} />
                      <span>Recent Searches</span>
                    </div>
                    <button
                      onClick={clearRecentSearches}
                      className="text-zinc-500 hover:text-red-400 text-xs"
                    >
                      Clear
                    </button>
                  </div>
                )}

                {state.query.length < 2 &&
                  state.recentSearches.map((result, index) => (
                    <button
                      key={`recent-${result.id}`}
                      ref={index === state.selectedIndex ? selectedItemRef : null}
                      onClick={() => handleSelect(result)}
                      className={`w-full text-left px-4 py-2 border-b border-zinc-800 last:border-b-0 transition-colors ${
                        index === state.selectedIndex
                          ? 'bg-blue-500/20 border-l-2 border-l-blue-500'
                          : 'hover:bg-zinc-800'
                      }`}
                    >
                      <div className="flex items-center gap-2">
                        <Clock size={14} className="text-zinc-500" />
                        <span className="text-white truncate">{result.title}</span>
                        {result.score !== undefined && (
                          <span className="ml-auto text-xs text-zinc-400">
                            {result.score.toFixed(1)}
                          </span>
                        )}
                      </div>
                    </button>
                  ))}
              </div>
            ) : state.isLoading ? (
              <div className="px-4 py-8 text-center">
                <Loader className="animate-spin mx-auto text-blue-500 mb-2" />
                <p className="text-sm text-zinc-400">Searching...</p>
              </div>
            ) : state.error ? (
              <div className="px-4 py-4 flex items-start gap-3">
                <AlertCircle className="text-red-400 flex-shrink-0 mt-0.5" size={16} />
                <div>
                  <p className="text-sm text-red-400">Search Error</p>
                  <p className="text-xs text-zinc-400 mt-1">{state.error}</p>
                </div>
              </div>
            ) : state.query.length >= 2 ? (
              <div className="px-4 py-8 text-center">
                <AlertCircle className="mx-auto text-zinc-500 mb-2" size={20} />
                <p className="text-sm text-zinc-400">No risks found</p>
              </div>
            ) : (
              <div className="px-4 py-6">
                <p className="text-sm text-zinc-400 mb-4">
                  Start typing to search risks or view recent searches
                </p>
                <div className="space-y-2">
                  <div className="text-xs text-zinc-500">
                    <div className="font-semibold mb-2">Keyboard Shortcuts:</div>
                    <ul className="space-y-1">
                      <li>
                        <kbd className="bg-zinc-800 px-2 py-1 rounded text-xs">
                          ↓↑
                        </kbd>{' '}
                        Navigate
                      </li>
                      <li>
                        <kbd className="bg-zinc-800 px-2 py-1 rounded text-xs">
                          Enter
                        </kbd>{' '}
                        Select
                      </li>
                      <li>
                        <kbd className="bg-zinc-800 px-2 py-1 rounded text-xs">
                          Esc
                        </kbd>{' '}
                        Close
                      </li>
                    </ul>
                  </div>
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
};

/**
 * Command Palette component for global commands
 */
interface CommandPaletteCommand {
  id: string;
  label: string;
  description?: string;
  action: () => void;
  shortcut?: string;
  category?: string;
}

interface CommandPaletteProps {
  commands: CommandPaletteCommand[];
}

export const CommandPalette: React.FC<CommandPaletteProps> = ({ commands }) => {
  const [isOpen, setIsOpen] = React.useState(false);
  const [search, setSearch] = React.useState('');
  const [selectedIndex, setSelectedIndex] = React.useState(0);

  useKeyboardShortcuts(() => {
    // Can trigger command palette here if needed
  });

  // Filter commands
  const filtered = commands.filter(
    (cmd) =>
      cmd.label.toLowerCase().includes(search.toLowerCase()) ||
      cmd.description?.toLowerCase().includes(search.toLowerCase())
  );

  const handleExecute = (command: CommandPaletteCommand) => {
    command.action();
    setIsOpen(false);
    setSearch('');
  };

  return (
    <>
      {/* Trigger Button */}
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center gap-2 px-3 py-2 bg-zinc-800 hover:bg-zinc-700 rounded-lg text-sm text-zinc-400 transition-colors"
      >
        <Zap size={16} />
        Commands
        <kbd className="ml-auto text-xs bg-zinc-900 px-2 py-0.5 rounded">⌘/</kbd>
      </button>

      {/* Command Palette Modal */}
      {isOpen && (
        <div className="fixed inset-0 bg-black/50 z-50 flex items-start justify-center pt-20">
          <div className="w-full max-w-lg bg-zinc-900 rounded-lg shadow-xl border border-zinc-700 overflow-hidden">
            {/* Search Input */}
            <div className="border-b border-zinc-700 p-3">
              <input
                autoFocus
                type="text"
                placeholder="Search commands..."
                value={search}
                onChange={(e) => {
                  setSearch(e.target.value);
                  setSelectedIndex(0);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'ArrowDown') {
                    setSelectedIndex((i) =>
                      Math.min(i + 1, filtered.length - 1)
                    );
                  } else if (e.key === 'ArrowUp') {
                    setSelectedIndex((i) => Math.max(i - 1, 0));
                  } else if (e.key === 'Enter' && filtered[selectedIndex]) {
                    handleExecute(filtered[selectedIndex]);
                  } else if (e.key === 'Escape') {
                    setIsOpen(false);
                  }
                }}
                className="w-full bg-transparent text-white outline-none text-sm"
              />
            </div>

            {/* Commands List */}
            <div className="max-h-80 overflow-y-auto">
              {filtered.length > 0 ? (
                filtered.map((cmd, index) => (
                  <button
                    key={cmd.id}
                    onClick={() => handleExecute(cmd)}
                    className={`w-full text-left px-4 py-3 border-b border-zinc-800 last:border-b-0 transition-colors ${
                      index === selectedIndex
                        ? 'bg-blue-500/20'
                        : 'hover:bg-zinc-800'
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <div>
                        <p className="text-white font-medium">{cmd.label}</p>
                        {cmd.description && (
                          <p className="text-xs text-zinc-400 mt-1">
                            {cmd.description}
                          </p>
                        )}
                      </div>
                      {cmd.shortcut && (
                        <kbd className="text-xs bg-zinc-800 px-2 py-1 rounded text-zinc-400">
                          {cmd.shortcut}
                        </kbd>
                      )}
                    </div>
                  </button>
                ))
              ) : (
                <div className="px-4 py-8 text-center text-zinc-400 text-sm">
                  No commands found
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  );
};
