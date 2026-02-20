import { useEffect, useState } from 'react';
import { Plus, Trash2, Edit2, Copy, AlertCircle, CheckCircle, Loader } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'react-hot-toast';

interface CustomField {
  id: string;
  name: string;
  type: 'TEXT' | 'NUMBER' | 'CHOICE' | 'DATE' | 'CHECKBOX';
  description: string;
  is_required: boolean;
  is_searchable: boolean;
  default_value?: string;
  scope: string;
  created_at: string;
  updated_at: string;
}

interface CustomFieldTemplate {
  id: string;
  name: string;
  description: string;
  fields: CustomField[];
  framework: string;
  created_at: string;
}

export default function CustomFields() {
  const [fields, setFields] = useState<CustomField[]>([]);
  const [templates, setTemplates] = useState<CustomFieldTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'fields' | 'templates'>('fields');
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showTemplateModal, setShowTemplateModal] = useState(false);
  const [editingField, setEditingField] = useState<CustomField | null>(null);

  const [formData, setFormData] = useState({
    name: '',
    type: 'TEXT' as const,
    description: '',
    is_required: false,
    is_searchable: true,
    default_value: '',
    scope: 'global',
  });

  const [templateForm, setTemplateForm] = useState({
    name: '',
    description: '',
    framework: 'ISO31000',
    fields: [] as string[],
  });

  const fieldTypes = [
    { value: 'TEXT', label: 'Text', icon: '📝' },
    { value: 'NUMBER', label: 'Number', icon: '🔢' },
    { value: 'CHOICE', label: 'Choice', icon: '📋' },
    { value: 'DATE', label: 'Date', icon: '📅' },
    { value: 'CHECKBOX', label: 'Checkbox', icon: '☑️' },
  ];

  const scopes = ['global', 'risk', 'mitigation', 'asset', 'incident'];
  const frameworks = ['ISO31000', 'NIST', 'CIS', 'Custom'];

  useEffect(() => {
    fetchFields();
    fetchTemplates();
  }, []);

  const fetchFields = async () => {
    try {
      const response = await fetch('/api/v1/custom-fields', {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
      });
      if (response.ok) {
        const data = await response.json();
        setFields(data || []);
      }
    } catch (error) {
      toast.error('Failed to fetch custom fields');
    } finally {
      setLoading(false);
    }
  };

  const fetchTemplates = async () => {
    try {
      const response = await fetch('/api/v1/custom-fields/templates', {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
      });
      if (response.ok) {
        const data = await response.json();
        setTemplates(data || []);
      }
    } catch (error) {
      console.error('Failed to fetch templates');
    }
  };

  const handleCreateField = async () => {
    if (!formData.name.trim()) {
      toast.error('Field name is required');
      return;
    }

    try {
      const url = editingField
        ? `/api/v1/custom-fields/${editingField.id}`
        : '/api/v1/custom-fields';
      const method = editingField ? 'PUT' : 'POST';

      const response = await fetch(url, {
        method,
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
        body: JSON.stringify(formData),
      });

      if (response.ok) {
        toast.success(editingField ? 'Field updated' : 'Field created');
        setShowCreateModal(false);
        setEditingField(null);
        setFormData({
          name: '',
          type: 'TEXT',
          description: '',
          is_required: false,
          is_searchable: true,
          default_value: '',
          scope: 'global',
        });
        fetchFields();
      } else {
        toast.error('Failed to save field');
      }
    } catch (error) {
      toast.error('Error saving field');
    }
  };

  const handleDeleteField = async (id: string) => {
    if (!confirm('Are you sure you want to delete this field?')) return;

    try {
      const response = await fetch(`/api/v1/custom-fields/${id}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
      });

      if (response.ok) {
        toast.success('Field deleted');
        fetchFields();
      } else {
        toast.error('Failed to delete field');
      }
    } catch (error) {
      toast.error('Error deleting field');
    }
  };

  const handleDuplicateField = (field: CustomField) => {
    setEditingField(null);
    setFormData({
      name: `${field.name} (Copy)`,
      type: field.type,
      description: field.description,
      is_required: field.is_required,
      is_searchable: field.is_searchable,
      default_value: field.default_value || '',
      scope: field.scope,
    });
    setShowCreateModal(true);
  };

  const handleEditField = (field: CustomField) => {
    setEditingField(field);
    setFormData({
      name: field.name,
      type: field.type,
      description: field.description,
      is_required: field.is_required,
      is_searchable: field.is_searchable,
      default_value: field.default_value || '',
      scope: field.scope,
    });
    setShowCreateModal(true);
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen bg-zinc-950">
        <Loader className="w-8 h-8 text-blue-500 animate-spin" />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-zinc-950 text-white p-6">
      <div className="max-w-6xl mx-auto">
        {/* Header */}
        <div className="flex justify-between items-center mb-8">
          <div>
            <h1 className="text-3xl font-bold">Custom Fields</h1>
            <p className="text-zinc-400 mt-2">Manage custom field definitions and templates</p>
          </div>
          <motion.button
            whileHover={{ scale: 1.05 }}
            whileTap={{ scale: 0.95 }}
            onClick={() => {
              setEditingField(null);
              setFormData({
                name: '',
                type: 'TEXT',
                description: '',
                is_required: false,
                is_searchable: true,
                default_value: '',
                scope: 'global',
              });
              setShowCreateModal(true);
            }}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded-lg transition"
          >
            <Plus className="w-5 h-5" />
            New Field
          </motion.button>
        </div>

        {/* Tabs */}
        <div className="flex gap-4 mb-6 border-b border-zinc-800">
          <button
            onClick={() => setActiveTab('fields')}
            className={`pb-3 px-4 font-medium transition ${
              activeTab === 'fields'
                ? 'border-b-2 border-blue-500 text-blue-500'
                : 'text-zinc-400 hover:text-white'
            }`}
          >
            Custom Fields ({fields.length})
          </button>
          <button
            onClick={() => setActiveTab('templates')}
            className={`pb-3 px-4 font-medium transition ${
              activeTab === 'templates'
                ? 'border-b-2 border-blue-500 text-blue-500'
                : 'text-zinc-400 hover:text-white'
            }`}
          >
            Templates ({templates.length})
          </button>
        </div>

        {/* Fields Tab */}
        {activeTab === 'fields' && (
          <div className="grid gap-4">
            <AnimatePresence>
              {fields.length === 0 ? (
                <motion.div
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  className="text-center py-12 bg-zinc-900 rounded-lg border border-zinc-800"
                >
                  <AlertCircle className="w-12 h-12 text-zinc-600 mx-auto mb-3" />
                  <p className="text-zinc-400">No custom fields yet</p>
                </motion.div>
              ) : (
                fields.map((field) => (
                  <motion.div
                    key={field.id}
                    layout
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -10 }}
                    className="bg-zinc-900 border border-zinc-800 rounded-lg p-4 hover:border-zinc-700 transition"
                  >
                    <div className="flex justify-between items-start">
                      <div className="flex-1">
                        <div className="flex items-center gap-3 mb-2">
                          <span className="text-2xl">
                            {fieldTypes.find((ft) => ft.value === field.type)?.icon}
                          </span>
                          <h3 className="text-lg font-semibold">{field.name}</h3>
                          <span className="px-2 py-1 bg-zinc-800 text-xs rounded text-zinc-300">
                            {field.type}
                          </span>
                          <span className="px-2 py-1 bg-zinc-800 text-xs rounded text-zinc-300">
                            {field.scope}
                          </span>
                        </div>
                        <p className="text-sm text-zinc-400 mb-2">{field.description}</p>
                        <div className="flex gap-4 text-xs text-zinc-500">
                          {field.is_required && (
                            <div className="flex items-center gap-1">
                              <CheckCircle className="w-4 h-4" />
                              Required
                            </div>
                          )}
                          {field.is_searchable && (
                            <div className="flex items-center gap-1">
                              <CheckCircle className="w-4 h-4" />
                              Searchable
                            </div>
                          )}
                          {field.default_value && (
                            <div>Default: {field.default_value}</div>
                          )}
                        </div>
                      </div>
                      <div className="flex gap-2 ml-4">
                        <motion.button
                          whileHover={{ scale: 1.1 }}
                          whileTap={{ scale: 0.9 }}
                          onClick={() => handleEditField(field)}
                          className="p-2 hover:bg-zinc-800 rounded transition"
                        >
                          <Edit2 className="w-4 h-4" />
                        </motion.button>
                        <motion.button
                          whileHover={{ scale: 1.1 }}
                          whileTap={{ scale: 0.9 }}
                          onClick={() => handleDuplicateField(field)}
                          className="p-2 hover:bg-zinc-800 rounded transition"
                        >
                          <Copy className="w-4 h-4" />
                        </motion.button>
                        <motion.button
                          whileHover={{ scale: 1.1 }}
                          whileTap={{ scale: 0.9 }}
                          onClick={() => handleDeleteField(field.id)}
                          className="p-2 hover:bg-red-900 rounded transition text-red-500"
                        >
                          <Trash2 className="w-4 h-4" />
                        </motion.button>
                      </div>
                    </div>
                  </motion.div>
                ))
              )}
            </AnimatePresence>
          </div>
        )}

        {/* Templates Tab */}
        {activeTab === 'templates' && (
          <div className="grid gap-4">
            {templates.length === 0 ? (
              <div className="text-center py-12 bg-zinc-900 rounded-lg border border-zinc-800">
                <AlertCircle className="w-12 h-12 text-zinc-600 mx-auto mb-3" />
                <p className="text-zinc-400">No templates available</p>
              </div>
            ) : (
              templates.map((template) => (
                <motion.div
                  key={template.id}
                  layout
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -10 }}
                  className="bg-zinc-900 border border-zinc-800 rounded-lg p-4 hover:border-zinc-700 transition"
                >
                  <div className="flex justify-between items-start mb-3">
                    <div>
                      <h3 className="text-lg font-semibold">{template.name}</h3>
                      <p className="text-sm text-zinc-400">{template.description}</p>
                      <span className="inline-block mt-2 px-2 py-1 bg-zinc-800 text-xs rounded text-zinc-300">
                        {template.framework}
                      </span>
                    </div>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {template.fields.map((field) => (
                      <span
                        key={field.id}
                        className="px-3 py-1 bg-blue-900/50 border border-blue-700/50 rounded-full text-sm text-blue-200"
                      >
                        {field.name}
                      </span>
                    ))}
                  </div>
                </motion.div>
              ))
            )}
          </div>
        )}
      </div>

      {/* Create/Edit Modal */}
      <AnimatePresence>
        {showCreateModal && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50"
            onClick={() => {
              setShowCreateModal(false);
              setEditingField(null);
            }}
          >
            <motion.div
              initial={{ scale: 0.95, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              exit={{ scale: 0.95, opacity: 0 }}
              onClick={(e) => e.stopPropagation()}
              className="bg-zinc-900 border border-zinc-800 rounded-lg p-6 max-w-md w-full max-h-[90vh] overflow-y-auto"
            >
              <h2 className="text-xl font-bold mb-4">
                {editingField ? 'Edit Field' : 'Create Custom Field'}
              </h2>

              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-zinc-300 mb-1">
                    Field Name *
                  </label>
                  <input
                    type="text"
                    value={formData.name}
                    onChange={(e) =>
                      setFormData({ ...formData, name: e.target.value })
                    }
                    className="w-full px-3 py-2 bg-zinc-800 border border-zinc-700 rounded text-white placeholder-zinc-500 focus:outline-none focus:border-blue-500"
                    placeholder="e.g., Department, Cost Center"
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-zinc-300 mb-2">
                    Field Type *
                  </label>
                  <div className="grid grid-cols-2 gap-2">
                    {fieldTypes.map((type) => (
                      <button
                        key={type.value}
                        onClick={() =>
                          setFormData({ ...formData, type: type.value as any })
                        }
                        className={`p-3 border rounded text-center transition ${
                          formData.type === type.value
                            ? 'bg-blue-600 border-blue-500'
                            : 'bg-zinc-800 border-zinc-700 hover:border-zinc-600'
                        }`}
                      >
                        <div className="text-xl mb-1">{type.icon}</div>
                        <div className="text-sm font-medium">{type.label}</div>
                      </button>
                    ))}
                  </div>
                </div>

                <div>
                  <label className="block text-sm font-medium text-zinc-300 mb-1">
                    Description
                  </label>
                  <textarea
                    value={formData.description}
                    onChange={(e) =>
                      setFormData({ ...formData, description: e.target.value })
                    }
                    className="w-full px-3 py-2 bg-zinc-800 border border-zinc-700 rounded text-white placeholder-zinc-500 focus:outline-none focus:border-blue-500 resize-none"
                    rows={3}
                    placeholder="Field description"
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-zinc-300 mb-1">
                    Scope
                  </label>
                  <select
                    value={formData.scope}
                    onChange={(e) =>
                      setFormData({ ...formData, scope: e.target.value })
                    }
                    className="w-full px-3 py-2 bg-zinc-800 border border-zinc-700 rounded text-white focus:outline-none focus:border-blue-500"
                  >
                    {scopes.map((scope) => (
                      <option key={scope} value={scope}>
                        {scope.charAt(0).toUpperCase() + scope.slice(1)}
                      </option>
                    ))}
                  </select>
                </div>

                <div className="flex items-center gap-3">
                  <input
                    type="checkbox"
                    id="required"
                    checked={formData.is_required}
                    onChange={(e) =>
                      setFormData({ ...formData, is_required: e.target.checked })
                    }
                    className="w-4 h-4 rounded"
                  />
                  <label htmlFor="required" className="text-sm text-zinc-300">
                    Required
                  </label>
                </div>

                <div className="flex items-center gap-3">
                  <input
                    type="checkbox"
                    id="searchable"
                    checked={formData.is_searchable}
                    onChange={(e) =>
                      setFormData({ ...formData, is_searchable: e.target.checked })
                    }
                    className="w-4 h-4 rounded"
                  />
                  <label htmlFor="searchable" className="text-sm text-zinc-300">
                    Searchable
                  </label>
                </div>

                {formData.type === 'TEXT' && (
                  <div>
                    <label className="block text-sm font-medium text-zinc-300 mb-1">
                      Default Value
                    </label>
                    <input
                      type="text"
                      value={formData.default_value}
                      onChange={(e) =>
                        setFormData({ ...formData, default_value: e.target.value })
                      }
                      className="w-full px-3 py-2 bg-zinc-800 border border-zinc-700 rounded text-white placeholder-zinc-500 focus:outline-none focus:border-blue-500"
                      placeholder="Optional default value"
                    />
                  </div>
                )}
              </div>

              <div className="flex gap-3 mt-6">
                <button
                  onClick={() => {
                    setShowCreateModal(false);
                    setEditingField(null);
                  }}
                  className="flex-1 px-4 py-2 bg-zinc-800 hover:bg-zinc-700 rounded transition"
                >
                  Cancel
                </button>
                <motion.button
                  whileHover={{ scale: 1.05 }}
                  whileTap={{ scale: 0.95 }}
                  onClick={handleCreateField}
                  className="flex-1 px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded transition font-medium"
                >
                  {editingField ? 'Update' : 'Create'}
                </motion.button>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
