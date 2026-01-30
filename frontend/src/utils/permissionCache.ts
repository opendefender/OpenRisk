/
  Permission Caching and Memoization Utilities
  Improves performance by caching permission check results
 /

export interface CacheEntry<T> {
  value: T;
  timestamp: number;
  ttl?: number; // Time to live in milliseconds
}

/
  Simple cache for permission check results
 /
export class PermissionCache {
  private cache = new Map<string, CacheEntry<boolean>>();
  private defaultTTL =     ; //  minutes default
  private maxSize = ; // Max cache entries

  /
    Get cached permission check result
   /
  get(key: string): boolean | null {
    const entry = this.cache.get(key);
    if (!entry) return null;

    // Check if expired
    if (entry.ttl) {
      const age = Date.now() - entry.timestamp;
      if (age > entry.ttl) {
        this.cache.delete(key);
        return null;
      }
    }

    return entry.value;
  }

  /
    Set cached permission check result
   /
  set(key: string, value: boolean, ttl?: number): void {
    // Prevent unbounded cache growth
    if (this.cache.size >= this.maxSize) {
      // Remove oldest entry
      const firstKey = this.cache.keys().next().value;
      if (firstKey) {
        this.cache.delete(firstKey);
      }
    }

    this.cache.set(key, {
      value,
      timestamp: Date.now(),
      ttl: ttl ?? this.defaultTTL,
    });
  }

  /
    Clear entire cache
   /
  clear(): void {
    this.cache.clear();
  }

  /
    Clear expired entries
   /
  clearExpired(): void {
    const now = Date.now();
    for (const [key, entry] of this.cache.entries()) {
      if (entry.ttl && now - entry.timestamp > entry.ttl) {
        this.cache.delete(key);
      }
    }
  }

  /
    Set default TTL for all future entries
   /
  setDefaultTTL(ttl: number): void {
    this.defaultTTL = ttl;
  }

  /
    Get cache size
   /
  size(): number {
    return this.cache.size;
  }

  /
    Get cache stats
   /
  getStats(): {
    size: number;
    maxSize: number;
    defaultTTL: number;
  } {
    return {
      size: this.cache.size,
      maxSize: this.maxSize,
      defaultTTL: this.defaultTTL,
    };
  }
}

/
  Create a memoized permission check function
  Caches results to avoid repeated checking
 /
export const memoizePermissionCheck = (
  fn: (permission: string) => boolean,
  cache?: PermissionCache
): ((permission: string) => boolean) => {
  const permCache = cache ?? new PermissionCache();

  return (permission: string): boolean => {
    const cached = permCache.get(permission);
    if (cached !== null) {
      return cached;
    }

    const result = fn(permission);
    permCache.set(permission, result);
    return result;
  };
};

/
  Batch permission check results for efficiency
  Returns a map of permission -> allowed
 /
export const batchCheckPermissions = (
  permissions: string[],
  checkFn: (permission: string) => boolean,
  cache?: PermissionCache
): Map<string, boolean> => {
  const result = new Map<string, boolean>();
  const permCache = cache ?? new PermissionCache();

  for (const permission of permissions) {
    const cached = permCache.get(permission);
    if (cached !== null) {
      result.set(permission, cached);
    } else {
      const allowed = checkFn(permission);
      permCache.set(permission, allowed);
      result.set(permission, allowed);
    }
  }

  return result;
};

/
  Debounce cache invalidation
  Useful when user permissions update to avoid thrashing cache clears
 /
export class DebouncedPermissionCache extends PermissionCache {
  private debounceTimer: NodeJS.Timeout | null = null;
  private debounceDelay = ; //  second

  /
    Clear cache with debounce
   /
  debouncedClear(): void {
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
    }

    this.debounceTimer = setTimeout(() => {
      this.clear();
      this.debounceTimer = null;
    }, this.debounceDelay);
  }

  /
    Set debounce delay
   /
  setDebounceDelay(delay: number): void {
    this.debounceDelay = delay;
  }

  /
    Cancel pending clear
   /
  cancelPendingClear(): void {
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
      this.debounceTimer = null;
    }
  }
}

/
  Global permission cache instance
 /
export const permissionCache = new DebouncedPermissionCache();

/
  React hook for getting cached permission check function
  Provides memoized permission checking with cache management
 /
export const useCachedPermissionCheck = (
  checkFn: (permission: string) => boolean
) => {
  const memoizedCheck = memoizePermissionCheck(checkFn, permissionCache);

  return {
    can: memoizedCheck,
    invalidateCache: () => permissionCache.debouncedClear(),
    cacheStats: () => permissionCache.getStats(),
  };
};

export default {
  PermissionCache,
  DebouncedPermissionCache,
  memoizePermissionCheck,
  batchCheckPermissions,
  permissionCache,
  useCachedPermissionCheck,
};
