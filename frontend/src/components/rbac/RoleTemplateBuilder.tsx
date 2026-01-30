import React, { useMemo, useState } from 'react';
import { ChevronRight, Copy, Plus, Shield, Trash } from 'lucide-react';
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
      name: ${template.name} (Copy),
    });
    onCreateCustom?.(cloned);
  };

  return (
    <div className="w-full max-w-xl mx-auto space-y-">
      {/ Template Selection /}
      <div className="bg-white rounded-lg shadow-sm border border-gray- p-">
        <div className="flex items-center gap- mb-">
          <Shield className="w- h- text-blue-" />
          <h className="text-lg font-semibold">Role Templates</h>
        </div>

        <div className="grid grid-cols- md:grid-cols- lg:grid-cols- gap-">
          {templates.map((template) => (
            <motion.button
              key={template.name}
              onClick={() => handleSelectTemplate(template)}
              className={p- rounded-lg border- transition-all text-left ${
                selectedTemplate?.name === template.name
                  ? 'border-blue- bg-blue-'
                  : 'border-gray- hover:border-gray-'
              }}
              whileHover={{ scale: . }}
              whileTap={{ scale: . }}
            >
              <div className="flex items-start justify-between mb-">
                <div>
                  <h className="font-semibold text-sm">{template.name}</h>
                  <p className="text-xs text-gray-">Level {template.level}</p>
                </div>
                {selectedTemplate?.name === template.name && (
                  <ChevronRight className="w- h- text-blue-" />
                )}
              </div>
              <p className="text-xs text-gray- mb-">{template.description}</p>
              <div className="flex items-center justify-between">
                <span className="text-xs bg-gray- px- py- rounded">
                  {template.permissions.length} perms
                </span>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    handleDuplicateTemplate(template);
                  }}
                  className="p- hover:bg-gray- rounded"
                  title="Duplicate template"
                >
                  <Copy className="w- h- text-gray-" />
                </button>
              </div>
            </motion.button>
          ))}
        </div>
      </div>

      {/ Template Details /}
      {selectedTemplate && (
        <motion.div
          initial={{ opacity: , y:  }}
          animate={{ opacity: , y:  }}
          className="bg-white rounded-lg shadow-sm border border-gray- p- space-y-"
        >
          <h className="font-semibold text-lg">{selectedTemplate.name} Details</h>

          {/ Features /}
          <div>
            <h className="text-sm font-medium text-gray- mb-">Features Enabled</h>
            <div className="flex flex-wrap gap-">
              {selectedTemplate.features.map((feature) => (
                <span
                  key={feature}
                  className="text-xs bg-green- text-green- px- py- rounded-full"
                >
                  {feature}
                </span>
              ))}
            </div>
          </div>

          {/ Permissions Grid /}
          <div>
            <h className="text-sm font-medium text-gray- mb-">Permissions</h>
            <div className="grid grid-cols- md:grid-cols- gap-">
              {selectedTemplate.permissions.map((permission) => (
                <div key={permission} className="flex items-center gap-">
                  <input
                    type="checkbox"
                    checked={!excludedPermissions.includes(permission)}
                    onChange={() => handleToggleExcludedPermission(permission)}
                    className="w- h-"
                  />
                  <span className="text-sm text-gray-">{permission}</span>
                </div>
              ))}
            </div>
          </div>

          {/ Add Custom Permissions /}
          <div>
            <h className="text-sm font-medium text-gray- mb-">Add Custom Permissions</h>
            <div className="flex gap-">
              <input
                type="text"
                placeholder="e.g., api-keys:create"
                className="flex- px- py- border border-gray- rounded-lg text-sm"
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
              <button className="px- py- bg-blue- text-white rounded-lg hover:bg-blue-">
                <Plus className="w- h-" />
              </button>
            </div>
            {customPermissions.length >  && (
              <div className="mt- space-y-">
                {customPermissions.map((perm) => (
                  <div key={perm} className="flex items-center justify-between bg-blue- p- rounded">
                    <span className="text-sm">{perm}</span>
                    <button
                      onClick={() => handleToggleCustomPermission(perm)}
                      className="p- hover:bg-blue- rounded"
                    >
                      <Trash className="w- h- text-red-" />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/ Final Permissions Summary /}
          <div className="bg-gray- p- rounded-lg">
            <p className="text-sm font-medium text-gray- mb-">
              Final Permissions ({finalPermissions.length})
            </p>
            <div className="flex flex-wrap gap-">
              {finalPermissions.map((perm) => (
                <span
                  key={perm}
                  className="text-xs bg-white border border-gray- px- py- rounded"
                >
                  {perm}
                </span>
              ))}
            </div>
          </div>

          {/ Action Buttons /}
          <div className="flex gap- pt-">
            <button
              onClick={handleCreateCustom}
              className="flex- px- py- bg-blue- text-white rounded-lg hover:bg-blue- transition-colors"
            >
              Create Custom Role
            </button>
          </div>
        </motion.div>
      )}

      {/ Template Comparison /}
      {showComparison && selectedTemplate && (
        <motion.div
          initial={{ opacity: , y:  }}
          animate={{ opacity: , y:  }}
          className="bg-white rounded-lg shadow-sm border border-gray- p-"
        >
          <h className="font-semibold text-lg mb-">Compare Templates</h>

          <div className="grid grid-cols- md:grid-cols- gap- mb-">
            {templates
              .filter((t) => t.name !== selectedTemplate.name)
              .map((template) => (
                <button
                  key={template.name}
                  onClick={() => setComparisonTemplate(template)}
                  className={p- rounded-lg border- transition-all text-left ${
                    comparisonTemplate?.name === template.name
                      ? 'border-purple- bg-purple-'
                      : 'border-gray- hover:border-gray-'
                  }}
                >
                  <div className="text-sm font-medium">{template.name}</div>
                  <div className="text-xs text-gray-">Level {template.level}</div>
                </button>
              ))}
          </div>

          {comparison && (
            <div className="grid grid-cols- md:grid-cols- gap-">
              <div className="bg-green- p- rounded-lg">
                <h className="text-sm font-medium text-green- mb-">Common ({comparison.commonPermissions.length})</h>
                <div className="space-y- max-h- overflow-y-auto">
                  {comparison.commonPermissions.map((perm) => (
                    <div key={perm} className="text-xs text-green-">{perm}</div>
                  ))}
                </div>
              </div>

              <div className="bg-blue- p- rounded-lg">
                <h className="text-sm font-medium text-blue- mb-">
                  Only in {selectedTemplate.name} ({comparison.onlyInTemplate.length})
                </h>
                <div className="space-y- max-h- overflow-y-auto">
                  {comparison.onlyInTemplate.map((perm) => (
                    <div key={perm} className="text-xs text-blue-">{perm}</div>
                  ))}
                </div>
              </div>

              <div className="bg-orange- p- rounded-lg">
                <h className="text-sm font-medium text-orange- mb-">
                  Only in {comparisonTemplate?.name} ({comparison.onlyInTemplate.length})
                </h>
                <div className="space-y- max-h- overflow-y-auto">
                  {comparison.onlyInTemplate.map((perm) => (
                    <div key={perm} className="text-xs text-orange-">{perm}</div>
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
