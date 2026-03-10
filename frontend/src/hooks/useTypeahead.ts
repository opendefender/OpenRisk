/**
 * Advanced Typeahead Hook
 * Provides intelligent search with fuzzy matching, recent searches, and keyboard navigation
 */

import { useEffect, useRef, useState, useCallback } from 'react';
import { searchRisks } from '@/api/risks';
import type { Risk } from '@/hooks/useRiskStore';

export interface TypeaheadConfig {
  minChars?: number;
  maxResults?: number;
  debounceMs?: number;
  storageKey?: string;
  enableFuzzyMatch?: boolean;
  enableRecentSearches?: boolean;
}

export interface TypeaheadResult {
  id: string;
  title: string;
  description?: string;
  score?: number;
  impact?: number;
  probability?: number;
  matchScore?: number; // 0-1 for fuzzy matching
  isRecent?: boolean;
  type: 'risk' | 'recent';
}

export interface TypeaheadState {
  query: string;
  results: TypeaheadResult[];
  recentSearches: TypeaheadResult[];
  selectedIndex: number;
  isLoading: boolean;
  isOpen: boolean;
  error?: string;
}

/**
 * Fuzzy matching score calculation (0-1)
 * Returns higher score for better matches
 */
function calculateFuzzyScore(query: string, text: string): number {
  const queryLower = query.toLowerCase();
  const textLower = text.toLowerCase();

  if (queryLower === textLower) return 1; // Exact match
  if (textLower.includes(queryLower)) return 0.9; // Substring match

  // Character-by-character matching with penalties
  let score = 0;
  let queryIndex = 0;
  let consecutiveMatches = 0;

  for (let i = 0; i < textLower.length && queryIndex < queryLower.length; i++) {
    if (textLower[i] === queryLower[queryIndex]) {
      score += 1 + consecutiveMatches * 0.5; // Bonus for consecutive matches
      queryIndex++;
      consecutiveMatches++;
    } else {
      consecutiveMatches = 0;
    }
  }

  // If we didn't match all query chars, return 0
  if (queryIndex < queryLower.length) return 0;

  // Normalize score based on text length
  return Math.min(1, score / textLower.length);
}

/**
 * Main useTypeahead hook
 */
export function useTypeahead(config: TypeaheadConfig = {}) {
  const {
    minChars = 2,
    maxResults = 10,
    debounceMs = 300,
    storageKey = 'openrisk_recent_searches',
    enableFuzzyMatch = true,
    enableRecentSearches = true,
  } = config;

  const [state, setState] = useState<TypeaheadState>({
    query: '',
    results: [],
    recentSearches: [],
    selectedIndex: -1,
    isLoading: false,
    isOpen: false,
  });

  const debounceTimer = useRef<NodeJS.Timeout>();
  const containerRef = useRef<HTMLDivElement>(null);
  const selectedItemRef = useRef<HTMLDivElement>(null);

  // Load recent searches from localStorage
  useEffect(() => {
    if (!enableRecentSearches) return;

    try {
      const stored = localStorage.getItem(storageKey);
      const recentSearches = stored ? JSON.parse(stored) : [];
      setState((prev) => ({ ...prev, recentSearches }));
    } catch (error) {
      console.error('Failed to load recent searches:', error);
    }
  }, [enableRecentSearches, storageKey]);

  // Save recent search to localStorage
  const saveRecentSearch = useCallback(
    (result: TypeaheadResult) => {
      if (!enableRecentSearches) return;

      try {
        const stored = localStorage.getItem(storageKey);
        let recent: TypeaheadResult[] = stored ? JSON.parse(stored) : [];

        // Remove if already exists
        recent = recent.filter((r) => r.id !== result.id);

        // Add to front
        recent.unshift({ ...result, isRecent: true, type: 'recent' });

        // Keep only last 10
        recent = recent.slice(0, 10);

        localStorage.setItem(storageKey, JSON.stringify(recent));
        setState((prev) => ({ ...prev, recentSearches: recent }));
      } catch (error) {
        console.error('Failed to save recent search:', error);
      }
    },
    [enableRecentSearches, storageKey]
  );

  // Debounced search function
  const performSearch = useCallback(
    async (query: string) => {
      if (query.length < minChars) {
        setState((prev) => ({
          ...prev,
          results: [],
          selectedIndex: -1,
          isOpen: false,
        }));
        return;
      }

      setState((prev) => ({ ...prev, isLoading: true, error: undefined }));

      try {
        // Fetch from API
        const response = await searchRisks({
          q: query,
          limit: Math.min(maxResults * 2, 50), // Get more, then filter
        });

        let results: TypeaheadResult[] = response.data.items.map(
          (risk: Risk) => ({
            id: risk.id,
            title: risk.title,
            description: risk.description,
            score: risk.score,
            impact: risk.impact,
            probability: risk.probability,
            matchScore: enableFuzzyMatch
              ? calculateFuzzyScore(query, risk.title)
              : 1,
            type: 'risk' as const,
          })
        );

        // Sort by fuzzy match score if enabled
        if (enableFuzzyMatch) {
          results.sort((a, b) => (b.matchScore || 0) - (a.matchScore || 0));
        }

        // Limit results
        results = results.slice(0, maxResults);

        setState((prev) => ({
          ...prev,
          results,
          isLoading: false,
          isOpen: results.length > 0,
          selectedIndex: results.length > 0 ? 0 : -1,
        }));
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : 'Search failed';
        setState((prev) => ({
          ...prev,
          isLoading: false,
          error: errorMessage,
          results: [],
        }));
      }
    },
    [minChars, maxResults, enableFuzzyMatch]
  );

  // Handle input change with debouncing
  const handleInput = useCallback(
    (query: string) => {
      setState((prev) => ({ ...prev, query, isOpen: query.length >= minChars }));

      if (debounceTimer.current) {
        clearTimeout(debounceTimer.current);
      }

      debounceTimer.current = setTimeout(() => {
        performSearch(query);
      }, debounceMs);
    },
    [debounceMs, minChars, performSearch]
  );

  // Handle keyboard navigation
  const handleKeyDown = useCallback(
    (event: React.KeyboardEvent) => {
      const { key } = event;
      const allResults = [
        ...(state.isOpen ? state.results : []),
        ...(state.isOpen && enableRecentSearches && state.query.length < minChars
          ? state.recentSearches
          : []),
      ];

      switch (key) {
        case 'ArrowDown':
          event.preventDefault();
          setState((prev) => ({
            ...prev,
            selectedIndex:
              prev.selectedIndex < allResults.length - 1
                ? prev.selectedIndex + 1
                : prev.selectedIndex,
          }));
          break;

        case 'ArrowUp':
          event.preventDefault();
          setState((prev) => ({
            ...prev,
            selectedIndex: prev.selectedIndex > 0 ? prev.selectedIndex - 1 : -1,
          }));
          break;

        case 'Enter':
          event.preventDefault();
          if (state.selectedIndex >= 0 && allResults[state.selectedIndex]) {
            const selected = allResults[state.selectedIndex];
            saveRecentSearch(selected);
            setState((prev) => ({
              ...prev,
              isOpen: false,
            }));
            // Return the selected result for parent component to handle
            return selected;
          }
          break;

        case 'Escape':
          event.preventDefault();
          setState((prev) => ({ ...prev, isOpen: false, selectedIndex: -1 }));
          break;

        default:
          break;
      }

      return null;
    },
    [state, enableRecentSearches, minChars, saveRecentSearch]
  );

  // Scroll selected item into view
  useEffect(() => {
    if (selectedItemRef.current) {
      selectedItemRef.current.scrollIntoView({
        block: 'nearest',
        behavior: 'smooth',
      });
    }
  }, [state.selectedIndex]);

  // Close on outside click
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setState((prev) => ({ ...prev, isOpen: false }));
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  return {
    state,
    handleInput,
    handleKeyDown,
    containerRef,
    selectedItemRef,
    saveRecentSearch,
    clearRecentSearches: () => {
      localStorage.removeItem(storageKey);
      setState((prev) => ({ ...prev, recentSearches: [] }));
    },
  };
}

/**
 * Keyboard shortcuts provider
 * Provides access to global shortcuts like Cmd+K for search
 */
export function useKeyboardShortcuts(onSearchFocus?: () => void) {
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      // Cmd+K or Ctrl+K to focus search
      if ((event.metaKey || event.ctrlKey) && event.key === 'k') {
        event.preventDefault();
        onSearchFocus?.();
      }

      // Cmd+/ or Ctrl+/ for help
      if ((event.metaKey || event.ctrlKey) && event.key === '/') {
        event.preventDefault();
        // Open help modal (to be implemented)
        console.log('Open help modal');
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [onSearchFocus]);
}
