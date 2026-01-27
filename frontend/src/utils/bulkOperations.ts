/**
 * Bulk Operations Utilities
 * Functions for handling batch permission and role operations
 */

export interface BulkOperation {
  id: string;
  type: 'grant' | 'revoke' | 'update' | 'delete';
  targetIds: string[];
  permissions?: string[];
  roleId?: string;
  metadata?: Record<string, any>;
  createdAt: Date;
  status: 'pending' | 'in-progress' | 'completed' | 'failed';
  results?: OperationResult[];
}

export interface OperationResult {
  targetId: string;
  success: boolean;
  message?: string;
  error?: string;
}

export interface BulkOperationStats {
  totalOperations: number;
  successful: number;
  failed: number;
  successRate: number;
  duration: number; // in milliseconds
}

/**
 * Create bulk operation
 */
export const createBulkOperation = (
  type: BulkOperation['type'],
  targetIds: string[],
  permissions?: string[],
  roleId?: string
): BulkOperation => {
  return {
    id: `bulk-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
    type,
    targetIds,
    permissions,
    roleId,
    createdAt: new Date(),
    status: 'pending',
    results: [],
  };
};

/**
 * Validate bulk operation
 */
export const validateBulkOperation = (op: BulkOperation): {
  valid: boolean;
  errors: string[];
} => {
  const errors: string[] = [];

  if (!op.targetIds || op.targetIds.length === 0) {
    errors.push('At least one target is required');
  }

  if (op.type === 'grant' || op.type === 'revoke') {
    if (!op.permissions || op.permissions.length === 0) {
      errors.push('At least one permission is required for grant/revoke');
    }
  }

  if (op.type === 'update') {
    if (!op.roleId) {
      errors.push('Role ID is required for update operations');
    }
  }

  return {
    valid: errors.length === 0,
    errors,
  };
};

/**
 * Parse CSV for bulk operations
 */
export const parseCSVForBulkOps = (
  csv: string,
  format: 'users' | 'roles' = 'users'
): { ids: string[]; headers: string[]; errors: string[] } => {
  const errors: string[] = [];
  const lines = csv.trim().split('\n');

  if (lines.length < 2) {
    errors.push('CSV must have at least a header and one data row');
    return { ids: [], headers: [], errors };
  }

  const headers = lines[0].split(',').map((h) => h.trim());
  const idColumn = format === 'users' ? 'user_id' : 'role_id';
  const idIndex = headers.indexOf(idColumn);

  if (idIndex === -1) {
    errors.push(`Column '${idColumn}' not found in CSV`);
    return { ids: [], headers, errors };
  }

  const ids: string[] = [];
  for (let i = 1; i < lines.length; i++) {
    const values = lines[i].split(',').map((v) => v.trim());
    if (values.length > idIndex && values[idIndex]) {
      ids.push(values[idIndex]);
    }
  }

  if (ids.length === 0) {
    errors.push('No valid IDs found in CSV');
  }

  return { ids, headers, errors };
};

/**
 * Chunk array for batch processing
 */
export const chunkArray = <T>(array: T[], chunkSize: number): T[][] => {
  const chunks: T[][] = [];
  for (let i = 0; i < array.length; i += chunkSize) {
    chunks.push(array.slice(i, i + chunkSize));
  }
  return chunks;
};

/**
 * Process bulk operation in batches
 */
export const processBulkOperationInBatches = async (
  op: BulkOperation,
  processFn: (targetId: string) => Promise<OperationResult>,
  batchSize: number = 10,
  onProgress?: (completed: number, total: number) => void
): Promise<BulkOperationStats> => {
  const startTime = Date.now();
  const batches = chunkArray(op.targetIds, batchSize);
  const results: OperationResult[] = [];
  let completed = 0;

  for (const batch of batches) {
    const batchResults = await Promise.allSettled(
      batch.map(async (targetId) => {
        try {
          return await processFn(targetId);
        } catch (error) {
          return {
            targetId,
            success: false,
            error: error instanceof Error ? error.message : 'Unknown error',
          };
        }
      })
    );

    for (const result of batchResults) {
      if (result.status === 'fulfilled') {
        results.push(result.value);
      }
      completed++;
      onProgress?.(completed, op.targetIds.length);
    }
  }

  const duration = Date.now() - startTime;
  const successful = results.filter((r) => r.success).length;
  const failed = results.filter((r) => !r.success).length;

  return {
    totalOperations: results.length,
    successful,
    failed,
    successRate: results.length > 0 ? (successful / results.length) * 100 : 0,
    duration,
  };
};

/**
 * Generate CSV export for operation results
 */
export const exportResultsAsCSV = (
  op: BulkOperation,
  stats: BulkOperationStats
): string => {
  const lines: string[] = [];

  // Header
  lines.push('Target ID,Status,Message');

  // Data rows
  op.results?.forEach((result) => {
    const status = result.success ? 'Success' : 'Failed';
    const message = result.message || result.error || '';
    lines.push(`${result.targetId},${status},"${message}"`);
  });

  // Summary
  lines.push('');
  lines.push(`Total,${stats.totalOperations}`);
  lines.push(`Successful,${stats.successful}`);
  lines.push(`Failed,${stats.failed}`);
  lines.push(`Success Rate,${stats.successRate.toFixed(2)}%`);
  lines.push(`Duration (ms),${stats.duration}`);

  return lines.join('\n');
};

/**
 * Create undo operation from original
 */
export const createUndoOperation = (
  originalOp: BulkOperation
): BulkOperation | null => {
  // Only grant operations can be easily undone
  if (originalOp.type !== 'grant') {
    return null;
  }

  return createBulkOperation(
    'revoke',
    originalOp.targetIds,
    originalOp.permissions,
    undefined
  );
};

/**
 * Get operation summary
 */
export const getOperationSummary = (op: BulkOperation): string => {
  const typeLabel = op.type.charAt(0).toUpperCase() + op.type.slice(1);
  const targetCount = op.targetIds.length;
  const successCount = op.results?.filter((r) => r.success).length || 0;
  const failCount = op.results?.filter((r) => !r.success).length || 0;

  if (op.status === 'pending' || op.status === 'in-progress') {
    return `${typeLabel} operation on ${targetCount} targets (${op.status})`;
  }

  return `${typeLabel} operation: ${successCount} successful, ${failCount} failed`;
};

/**
 * Check if operation can be retried
 */
export const canRetryOperation = (op: BulkOperation): boolean => {
  return (
    op.status === 'failed' &&
    (op.results?.some((r) => !r.success) || op.results?.length === 0)
  );
};

/**
 * Create retry operation for failed items
 */
export const createRetryOperation = (
  originalOp: BulkOperation
): BulkOperation | null => {
  const failedIds = originalOp.results
    ?.filter((r) => !r.success)
    .map((r) => r.targetId);

  if (!failedIds || failedIds.length === 0) {
    return null;
  }

  return createBulkOperation(
    originalOp.type,
    failedIds,
    originalOp.permissions,
    originalOp.roleId
  );
};

/**
 * Merge multiple operations
 */
export const mergeOperations = (ops: BulkOperation[]): BulkOperation => {
  const firstOp = ops[0];
  return {
    ...firstOp,
    id: `merged-${Date.now()}`,
    targetIds: Array.from(new Set(ops.flatMap((op) => op.targetIds))),
    results: ops.flatMap((op) => op.results || []),
  };
};

/**
 * Filter operations by criteria
 */
export const filterOperations = (
  operations: BulkOperation[],
  criteria: {
    type?: BulkOperation['type'];
    status?: BulkOperation['status'];
    minTargets?: number;
    maxTargets?: number;
    afterDate?: Date;
    beforeDate?: Date;
  }
): BulkOperation[] => {
  return operations.filter((op) => {
    if (criteria.type && op.type !== criteria.type) return false;
    if (criteria.status && op.status !== criteria.status) return false;
    if (
      criteria.minTargets &&
      op.targetIds.length < criteria.minTargets
    )
      return false;
    if (
      criteria.maxTargets &&
      op.targetIds.length > criteria.maxTargets
    )
      return false;
    if (criteria.afterDate && op.createdAt < criteria.afterDate) return false;
    if (criteria.beforeDate && op.createdAt > criteria.beforeDate) return false;
    return true;
  });
};

/**
 * Get operation statistics
 */
export const getOperationStats = (operations: BulkOperation[]): {
  totalOperations: number;
  byType: Record<string, number>;
  byStatus: Record<string, number>;
  totalTargets: number;
  averageTargetsPerOp: number;
} => {
  return {
    totalOperations: operations.length,
    byType: {
      grant: operations.filter((op) => op.type === 'grant').length,
      revoke: operations.filter((op) => op.type === 'revoke').length,
      update: operations.filter((op) => op.type === 'update').length,
      delete: operations.filter((op) => op.type === 'delete').length,
    },
    byStatus: {
      pending: operations.filter((op) => op.status === 'pending').length,
      'in-progress': operations.filter((op) => op.status === 'in-progress')
        .length,
      completed: operations.filter((op) => op.status === 'completed').length,
      failed: operations.filter((op) => op.status === 'failed').length,
    },
    totalTargets: operations.reduce((sum, op) => sum + op.targetIds.length, 0),
    averageTargetsPerOp:
      operations.length > 0
        ? operations.reduce((sum, op) => sum + op.targetIds.length, 0) /
          operations.length
        : 0,
  };
};

export default {
  createBulkOperation,
  validateBulkOperation,
  parseCSVForBulkOps,
  chunkArray,
  processBulkOperationInBatches,
  exportResultsAsCSV,
  createUndoOperation,
  getOperationSummary,
  canRetryOperation,
  createRetryOperation,
  mergeOperations,
  filterOperations,
  getOperationStats,
};
