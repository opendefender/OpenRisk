import React, { useMemo, useState } from 'react';
import { ChevronRight, Copy, Plus, Shield, Trash2 } from 'lucide-react';
import { getAllTemplates, compareTemplates, getRecommendedTemplate, cloneTemplate } from '../../utils/roleTemplateUtils';
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
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5 text-blue-600" />
          <h2 className="text-lg font-semibold">Role Templates</h2>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {templates.map((template) => (
            <motion.button
              key={template.name}
              onClick={() => handleSelectTemplate(template)}
              className={`p-4 rounded-lg border-2 transition-all text-left ${
                selectedTemplate?.name === template.name
                  ? 'border-blue-500 bg-blue-50'
                  : 'border-gray-200 hover:border-gray-300'
              }`}
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
            >
              <div className="flex items-start justify-between mb-2">
                <div>
                  <h3 className="font-semibold text-sm">{template.name}</h3>
                  <p className="text-xs text-gray-600">Level {template.level}</p>
                </div>
                {selectedTemplate?.name === template.name && (
                  <ChevronRight className="w-4 h-4 text-blue-600" />
                )}
              </div>
              <p className="text-xs text-gray-700 mb-2">{template.description}</p>
              <div className="flex items-center justify-between">
                <span className="text-xs bg-gray-100 px-2 py-1 rounded">
                  {template.permissions.length} perms
                </span>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    handleDuplicateTemplate(template);
                  }}
                  className="p-1 hover:bg-gray-200 rounded"
                  title="Duplicate template"
                >
                  <Copy className="w-4 h-4 text-gray-600" />
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
          className="bg-white rounded-lg shadow-sm border border-gray-200 p-6 space-y-4"
        >
          <h3 className="font-semibold text-lg">{selectedTemplate.name} Details</h3>

          {/* Features */}
          <div>
            <h4 className="text-sm font-medium text-gray-700 mb-2">Features Enabled</h4>
            <div className="flex flex-wrap gap-2">
              {selectedTemplate.features.map((feature) => (
                <span
                  key={feature}
                  className="text-xs bg-green-100 text-green-800 px-3 py-1 rounded-full"
                >
                  {feature}
                </span>
              ))}
            </div>
          </div>

          {/* Permissions Grid */}
          <div>
            <h4 className="text-sm font-medium text-gray-700 mb-3">Permissions</h4>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
              {selectedTemplate.permissions.map((permission) => (
                <div key={permission} className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={!excludedPermissions.includes(permission)}
                    onChange={() => handleToggleExcludedPermission(permission)}
                    className="w-4 h-4"
                  />
                  <span className="text-sm text-gray-700">{permission}</span>
                </div>
              ))}
            </div>
          </div>

          {/* Add Custom Permissions */}
          <div>
            <h4 className="text-sm font-medium text-gray-700 mb-2">Add Custom Permissions</h4>
            <div className="flex gap-2">
              <input
                type="text"
                placeholder="e.g., api-keys:create"
                className="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm"
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
              <button className="px-3 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700">
                <Plus className="w-4 h-4" />
              </button>
            </div>
            {customPermissions.length > 0 && (
              <div className="mt-3 space-y-2">
                {customPermissions.map((perm) => (
                  <div key={perm} className="flex items-center justify-between bg-blue-50 p-2 rounded">
                    <span className="text-sm">{perm}</span>
                    <button
                      onClick={() => handleToggleCustomPermission(perm)}
                      className="p-1 hover:bg-blue-200 rounded"
                    >
                      <Trash2 className="w-4 h-4 text-red-600" />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Final Permissions Summary */}
          <div className="bg-gray-50 p-4 rounded-lg">
            <p className="text-sm font-medium text-gray-700 mb-2">
              Final Permissions ({finalPermissions.length})
            </p>
            <div className="flex flex-wrap gap-2">
              {finalPermissions.map((perm) => (
                <span
                  key={perm}
                  className="text-xs bg-white border border-gray-300 px-2 py-1 rounded"
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
              className="flex-1 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
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
          className="bg-white rounded-lg shadow-sm border border-gray-200 p-6"
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
                      : 'border-gray-200 hover:border-gray-300'
                  }`}
                >
                  <div className="text-sm font-medium">{template.name}</div>
                  <div className="text-xs text-gray-600">Level {template.level}</div>
                </button>
              ))}
          </div>

          {comparison && (
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div className="bg-green-50 p-4 rounded-lg">
                <h4 className="text-sm font-medium text-green-900 mb-2">Common ({comparison.commonPermissions.length})</h4>
                <div className="space-y-1 max-h-40 overflow-y-auto">
                  {comparison.commonPermissions.map((perm) => (
                    <div key={perm} className="text-xs text-green-800">{perm}</div>
                  ))}
                </div>
              </div>

              <div className="bg-blue-50 p-4 rounded-lg">
                <h4 className="text-sm font-medium text-blue-900 mb-2">
                  Only in {selectedTemplate.name} ({comparison.onlyInTemplate1.length})
                </h4>
                <div className="space-y-1 max-h-40 overflow-y-auto">
                  {comparison.onlyInTemplate1.map((perm) => (
                    <div key={perm} className="text-xs text-blue-800">{perm}</div>
                  ))}
                </div>
              </div>

              <div className="bg-orange-50 p-4 rounded-lg">
                <h4 className="text-sm font-medium text-orange-900 mb-2">
                  Only in {comparisonTemplate?.name} ({comparison.onlyInTemplate2.length})
                </h4>
                <div className="space-y-1 max-h-40 overflow-y-auto">
                  {comparison.onlyInTemplate2.map((perm) => (
                    <div key={perm} className="text-xs text-orange-800">{perm}</div>
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
