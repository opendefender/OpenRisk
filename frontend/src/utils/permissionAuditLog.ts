/**
 * Permission Audit Logging Utilities
 * Tracks and logs permission-related activities for compliance and debugging
 */

export interface PermissionAuditEvent {
  id: string;
  timestamp: Date;
  userId: string;
  action: 'check' | 'grant' | 'revoke' | 'deny' | 'grant_failed';
  resource: string;
  permissionAction: string;
  permission: string;
  allowed: boolean;
  reason?: string;
  metadata?: Record<string, any>;
}

export interface AuditLog {
  events: PermissionAuditEvent[];
  startTime: Date;
  endTime?: Date;
}

/**
 * In-memory audit log store
 * In production, should be sent to a backend audit logging service
 */
class PermissionAuditLogger {
  private logs: PermissionAuditEvent[] = [];
  private maxLogs = 1000; // Prevent unlimited memory growth
  private enabled = process.env.NODE_ENV === 'development';

  /**
   * Log a permission check
   */
  log(event: Omit<PermissionAuditEvent, 'id' | 'timestamp'>): void {
    if (!this.enabled) return;

    const auditEvent: PermissionAuditEvent = {
      ...event,
      id: `audit-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
      timestamp: new Date(),
    };

    this.logs.push(auditEvent);

    // Keep only the most recent logs
    if (this.logs.length > this.maxLogs) {
      this.logs = this.logs.slice(-this.maxLogs);
    }

    // Log to console in development
    if (process.env.NODE_ENV === 'development') {
      console.debug('[Permission Audit]', {
        action: auditEvent.action,
        permission: auditEvent.permission,
        allowed: auditEvent.allowed,
        timestamp: auditEvent.timestamp,
      });
    }
  }

  /**
   * Log a permission check
   */
  logCheck(
    userId: string,
    permission: string,
    allowed: boolean,
    metadata?: Record<string, any>
  ): void {
    const [resource, action] = permission.split(':');
    this.log({
      userId,
      action: allowed ? 'check' : 'deny',
      resource: resource || 'unknown',
      permissionAction: action || 'unknown',
      permission,
      allowed,
      metadata,
    });
  }

  /**
   * Log a permission grant
   */
  logGrant(
    userId: string,
    targetUserId: string,
    permission: string,
    reason?: string
  ): void {
    const [resource, action] = permission.split(':');
    this.log({
      userId,
      action: 'grant',
      resource: resource || 'unknown',
      permissionAction: action || 'unknown',
      permission,
      allowed: true,
      reason,
      metadata: { targetUserId },
    });
  }

  /**
   * Log a permission revoke
   */
  logRevoke(
    userId: string,
    targetUserId: string,
    permission: string,
    reason?: string
  ): void {
    const [resource, action] = permission.split(':');
    this.log({
      userId,
      action: 'revoke',
      resource: resource || 'unknown',
      permissionAction: action || 'unknown',
      permission,
      allowed: true,
      reason,
      metadata: { targetUserId },
    });
  }

  /**
   * Log a failed permission grant attempt
   */
  logGrantFailed(
    userId: string,
    targetUserId: string,
    permission: string,
    reason: string
  ): void {
    const [resource, action] = permission.split(':');
    this.log({
      userId,
      action: 'grant_failed',
      resource: resource || 'unknown',
      permissionAction: action || 'unknown',
      permission,
      allowed: false,
      reason,
      metadata: { targetUserId },
    });
  }

  /**
   * Get all audit events
   */
  getEvents(): PermissionAuditEvent[] {
    return [...this.logs];
  }

  /**
   * Filter events by criteria
   */
  filterEvents(criteria: {
    userId?: string;
    permission?: string;
    action?: string;
    allowed?: boolean;
    startTime?: Date;
    endTime?: Date;
  }): PermissionAuditEvent[] {
    return this.logs.filter((event) => {
      if (criteria.userId && event.userId !== criteria.userId) return false;
      if (criteria.permission && event.permission !== criteria.permission)
        return false;
      if (criteria.action && event.action !== criteria.action) return false;
      if (criteria.allowed !== undefined && event.allowed !== criteria.allowed)
        return false;
      if (criteria.startTime && event.timestamp < criteria.startTime)
        return false;
      if (criteria.endTime && event.timestamp > criteria.endTime) return false;
      return true;
    });
  }

  /**
   * Clear all audit events
   */
  clear(): void {
    this.logs = [];
  }

  /**
   * Get summary statistics
   */
  getStats(): {
    totalEvents: number;
    deniedCount: number;
    grantCount: number;
    revokeCount: number;
    failedCount: number;
    uniqueUsers: number;
    uniquePermissions: number;
  } {
    return {
      totalEvents: this.logs.length,
      deniedCount: this.logs.filter((e) => e.action === 'deny').length,
      grantCount: this.logs.filter((e) => e.action === 'grant').length,
      revokeCount: this.logs.filter((e) => e.action === 'revoke').length,
      failedCount: this.logs.filter((e) => e.action === 'grant_failed').length,
      uniqueUsers: new Set(this.logs.map((e) => e.userId)).size,
      uniquePermissions: new Set(this.logs.map((e) => e.permission)).size,
    };
  }

  /**
   * Export logs as JSON
   */
  export(): string {
    return JSON.stringify(
      {
        exportDate: new Date(),
        events: this.logs,
        stats: this.getStats(),
      },
      null,
      2
    );
  }

  /**
   * Enable or disable audit logging
   */
  setEnabled(enabled: boolean): void {
    this.enabled = enabled;
  }

  /**
   * Set maximum number of logs to keep
   */
  setMaxLogs(max: number): void {
    this.maxLogs = Math.max(1, max);
    if (this.logs.length > this.maxLogs) {
      this.logs = this.logs.slice(-this.maxLogs);
    }
  }
}

// Export singleton instance
export const auditLogger = new PermissionAuditLogger();

/**
 * Hook for using audit logger in components
 */
export const useAuditLog = () => {
  return {
    log: auditLogger.logCheck.bind(auditLogger),
    grant: auditLogger.logGrant.bind(auditLogger),
    revoke: auditLogger.logRevoke.bind(auditLogger),
    grantFailed: auditLogger.logGrantFailed.bind(auditLogger),
    getEvents: () => auditLogger.getEvents(),
    getStats: () => auditLogger.getStats(),
    clear: () => auditLogger.clear(),
  };
};

export default {
  auditLogger,
  useAuditLog,
  PermissionAuditLogger,
};
