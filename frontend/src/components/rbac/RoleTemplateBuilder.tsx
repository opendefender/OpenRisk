// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import React, { useMemo, useState } from 'react';
import { ChevronRight, Copy, Plus, Shield, Trash2 } from 'lucide-react';
import { getAllTemplates, compareTemplates, cloneTemplate } from '../../utils/roleTemplateUtils';
import { motion } from 'framer-motion';
import type { RoleTemplate } from '../../utils/roleTemplateUtils';

interface RoleTemplateBuilderProps {
  onSelectTemplate?: (template: RoleTemplate) => void;
  onCreateCustom?: (template: RoleTemplate) => void;
  showComparison?: boolean;
}

export const RoleTemplateBuilder: React.FC<RoleTemplateBuilderProps> = ({
  onSelectTemplate,
  onCreateCustom,
  showComparison = true,
}) => {
  const templates = useMemo(() => getAllTemplates().sort((a, b) => a.level - b.level), []);
  const [selectedTemplate, setSelectedTemplate] = useState<RoleTemplate | null>(null);
  const [comparisonTemplate, setComparisonTemplate] = useState<RoleTemplate | null>(null);
  const [customPermissions, setCustomPermissions] = useState<string[]>([]);
  const [excludedPermissions, setExcludedPermissions] = useState<string[]>([]);

  const comparison = useMemo(() => {
    if (!selectedTemplate || !comparisonTemplate) return null;
    return compareTemplates(selectedTemplate, comparisonTemplate);
  }, [selectedTemplate, comparisonTemplate]);

  const finalPermissions = useMemo(() => {
    if (!selectedTemplate) return [];
    const basePerms = selectedTemplate.permissions.filter(
      (p) => !excludedPermissions.includes(p)
    );
    return Array.from(new Set([...basePerms, ...customPermissions]));
  }, [selectedTemplate, customPermissions, excludedPermissions]);

  const handleSelectTemplate = (template: RoleTemplate) => {
    setSelectedTemplate(template);
    setCustomPermissions([]);
    setExcludedPermissions([]);
    onSelectTemplate?.(template);
  };

  const handleToggleCustomPermission = (permission: string) => {
    if (customPermissions.includes(permission)) {
      setCustomPermissions(customPermissions.filter((p) => p !== permission));
    } else {
      setCustomPermissions([...customPermissions, permission]);
    }
  };

  const handleToggleExcludedPermission = (permission: string) => {
    if (excludedPermissions.includes(permission)) {
      setExcludedPermissions(excludedPermissions.filter((p) => p !== permission));
    } else {
      setExcludedPermissions([...excludedPermissions, permission]);
    }
  };

  const handleCreateCustom = () => {
    if (!selectedTemplate) return;
    const customRole = cloneTemplate(selectedTemplate, {
      permissions: finalPermissions,
    });
    onCreateCustom?.(customRole);
  };

  const handleDuplicateTemplate = (template: RoleTemplate) => {
    const cloned = cloneTemplate(template, {
      name: `${template.name} (Copy)`,
    });
    onCreateCustom?.(cloned);
  };

  return (
    <div className="w-full max-w-6xl mx-auto space-y-6">
      {/* Template Selection */}
      <div className="bg-surface-1 rounded-lg shadow-sm border border-border-subtle p-6">
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5 text-info-text" />
          <h2 className="text-lg font-semibold">Role Templates</h2>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {templates.map((template) => (
            <motion.button
              key={template.name}
              onClick={() => handleSelectTemplate(template)}
              className={`p-4 rounded-lg border-2 transition-all text-left ${
                selectedTemplate?.name === template.name
                  ? 'border-accent-400 bg-info-surface'
                  : 'border-border-subtle hover:border-border-subtle'
              }`}
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
            >
              <div className="flex items-start justify-between mb-2">
                <div>
                  <h3 className="font-semibold text-sm">{template.name}</h3>
                  <p className="text-xs text-text-muted">Level {template.level}</p>
                </div>
                {selectedTemplate?.name === template.name && (
                  <ChevronRight className="w-4 h-4 text-info-text" />
                )}
              </div>
              <p className="text-xs text-text-primary mb-2">{template.description}</p>
              <div className="flex items-center justify-between">
                <span className="text-xs bg-surface-sunken px-2 py-1 rounded">
                  {template.permissions.length} perms
                </span>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    handleDuplicateTemplate(template);
                  }}
                  className="p-1 hover:bg-surface-sunken rounded"
                  title="Duplicate template"
                >
                  <Copy className="w-4 h-4 text-text-muted" />
                </button>
              </div>
            </motion.button>
          ))}
        </div>
      </div>

      {/* Template Details */}
      {selectedTemplate && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className="bg-surface-1 rounded-lg shadow-sm border border-border-subtle p-6 space-y-4"
        >
          <h3 className="font-semibold text-lg">{selectedTemplate.name} Details</h3>

          {/* Features */}
          <div>
            <h4 className="text-sm font-medium text-text-primary mb-2">Features Enabled</h4>
            <div className="flex flex-wrap gap-2">
              {selectedTemplate.features.map((feature) => (
                <span
                  key={feature}
                  className="text-xs bg-success-surface text-success-text px-3 py-1 rounded-full"
                >
                  {feature}
                </span>
              ))}
            </div>
          </div>

          {/* Permissions Grid */}
          <div>
            <h4 className="text-sm font-medium text-text-primary mb-3">Permissions</h4>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
              {selectedTemplate.permissions.map((permission) => (
                <div key={permission} className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={!excludedPermissions.includes(permission)}
                    onChange={() => handleToggleExcludedPermission(permission)}
                    className="w-4 h-4"
                  />
                  <span className="text-sm text-text-primary">{permission}</span>
                </div>
              ))}
            </div>
          </div>

          {/* Add Custom Permissions */}
          <div>
            <h4 className="text-sm font-medium text-text-primary mb-2">Add Custom Permissions</h4>
            <div className="flex gap-2">
              <input
                type="text"
                placeholder="e.g., api-keys:create"
                className="flex-1 px-3 py-2 border border-border-subtle rounded-lg text-sm"
                onKeyPress={(e) => {
                  if (e.key === 'Enter' && e.currentTarget.value) {
                    const perm = e.currentTarget.value.trim();
                    if (!customPermissions.includes(perm)) {
                      handleToggleCustomPermission(perm);
                    }
                    e.currentTarget.value = '';
                  }
                }}
              />
              <button className="px-3 py-2 bg-accent-500 text-text-primary rounded-lg hover:bg-accent-500">
                <Plus className="w-4 h-4" />
              </button>
            </div>
            {customPermissions.length > 0 && (
              <div className="mt-3 space-y-2">
                {customPermissions.map((perm) => (
                  <div key={perm} className="flex items-center justify-between bg-info-surface p-2 rounded">
                    <span className="text-sm">{perm}</span>
                    <button
                      onClick={() => handleToggleCustomPermission(perm)}
                      className="p-1 hover:bg-info-surface rounded"
                    >
                      <Trash2 className="w-4 h-4 text-danger-text" />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Final Permissions Summary */}
          <div className="bg-surface-sunken p-4 rounded-lg">
            <p className="text-sm font-medium text-text-primary mb-2">
              Final Permissions ({finalPermissions.length})
            </p>
            <div className="flex flex-wrap gap-2">
              {finalPermissions.map((perm) => (
                <span
                  key={perm}
                  className="text-xs bg-surface-1 border border-border-subtle px-2 py-1 rounded"
                >
                  {perm}
                </span>
              ))}
            </div>
          </div>

          {/* Action Buttons */}
          <div className="flex gap-2 pt-4">
            <button
              onClick={handleCreateCustom}
              className="flex-1 px-4 py-2 bg-accent-500 text-text-primary rounded-lg hover:bg-accent-500 transition-colors"
            >
              Create Custom Role
            </button>
          </div>
        </motion.div>
      )}

      {/* Template Comparison */}
      {showComparison && selectedTemplate && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className="bg-surface-1 rounded-lg shadow-sm border border-border-subtle p-6"
        >
          <h3 className="font-semibold text-lg mb-4">Compare Templates</h3>

          <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-4">
            {templates
              .filter((t) => t.name !== selectedTemplate.name)
              .map((template) => (
                <button
                  key={template.name}
                  onClick={() => setComparisonTemplate(template)}
                  className={`p-3 rounded-lg border-2 transition-all text-left ${
                    comparisonTemplate?.name === template.name
                      ? 'border-purple-500 bg-purple-50'
                      : 'border-border-subtle hover:border-border-subtle'
                  }`}
                >
                  <div className="text-sm font-medium">{template.name}</div>
                  <div className="text-xs text-text-muted">Level {template.level}</div>
                </button>
              ))}
          </div>

          {comparison && (
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div className="bg-success-surface p-4 rounded-lg">
                <h4 className="text-sm font-medium text-success-text mb-2">Common ({comparison.commonPermissions.length})</h4>
                <div className="space-y-1 max-h-40 overflow-y-auto">
                  {comparison.commonPermissions.map((perm) => (
                    <div key={perm} className="text-xs text-success-text">{perm}</div>
                  ))}
                </div>
              </div>

              <div className="bg-info-surface p-4 rounded-lg">
                <h4 className="text-sm font-medium text-info-text mb-2">
                  Only in {selectedTemplate.name} ({comparison.onlyInTemplate1.length})
                </h4>
                <div className="space-y-1 max-h-40 overflow-y-auto">
                  {comparison.onlyInTemplate1.map((perm) => (
                    <div key={perm} className="text-xs text-info-text">{perm}</div>
                  ))}
                </div>
              </div>

              <div className="bg-warning-surface p-4 rounded-lg">
                <h4 className="text-sm font-medium text-warning-text mb-2">
                  Only in {comparisonTemplate?.name} ({comparison.onlyInTemplate2.length})
                </h4>
                <div className="space-y-1 max-h-40 overflow-y-auto">
                  {comparison.onlyInTemplate2.map((perm) => (
                    <div key={perm} className="text-xs text-warning-text">{perm}</div>
                  ))}
                </div>
              </div>
            </div>
          )}
        </motion.div>
      )}
    </div>
  );
};

export default RoleTemplateBuilder;
