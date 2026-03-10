import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Plus, Edit2, Trash2, Save, X, Settings } from 'lucide-react';
import { toast } from 'sonner';
import {
  getScoringConfigs,
  getScoringConfig,
  createScoringConfig,
  updateScoringConfig,
  ScoringConfig,
} from '../../../api/scoreEngineService';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';

export const ScoreEngineConfiguration = () => {
  const [configs, setConfigs] = useState<any>(null);
  const [selectedConfig, setSelectedConfig] = useState<ScoringConfig | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [formData, setFormData] = useState<Partial<ScoringConfig>>({});

  // Charger les configurations au montage
  useEffect(() => {
    loadConfigs();
  }, []);

  const loadConfigs = async () => {
    setIsLoading(true);
    try {
      const response = await getScoringConfigs();
      if (response.data) {
        setConfigs(response.data);
        setSelectedConfig(response.data.default);
      } else {
        toast.error('Erreur', { description: response.error || 'Impossible de charger les configurations' });
      }
    } catch (error) {
      console.error('Error loading configs:', error);
      toast.error('Erreur', { description: 'Impossible de charger les configurations' });
    } finally {
      setIsLoading(false);
    }
  };

  const handleEdit = (config: ScoringConfig) => {
    setSelectedConfig(config);
    setFormData({ ...config });
    setIsEditing(true);
  };

  const handleCreateNew = () => {
    setFormData({
      id: '',
      name: '',
      description: '',
      base_formula: 'impact*probability',
      weighting_factors: {
        impact: 1.0,
        probability: 1.0,
        criticality: 1.0,
        trend: 0.0,
      },
      risk_matrix_thresholds: {
        low: 5,
        medium: 12,
        high: 19,
        critical: 20,
      },
      asset_criticality_mult: {
        low: 0.8,
        medium: 1.0,
        high: 1.25,
        critical: 1.5,
      },
    });
    setSelectedConfig(null);
    setIsEditing(true);
  };

  const handleSave = async () => {
    if (!formData.name || !formData.id) {
      toast.error('Erreur', { description: 'Veuillez remplir tous les champs requis' });
      return;
    }

    setIsLoading(true);
    try {
      if (selectedConfig) {
        const response = await updateScoringConfig(selectedConfig.id, formData);
        if (response.error) {
          toast.error('Erreur', { description: response.error });
        } else {
          toast.success('Succès', { description: 'Configuration mise à jour' });
        }
      } else {
        const response = await createScoringConfig(formData);
        if (response.error) {
          toast.error('Erreur', { description: response.error });
        } else {
          toast.success('Succès', { description: 'Configuration créée' });
        }
      }
      setIsEditing(false);
      loadConfigs();
    } catch (error) {
      console.error('Error saving config:', error);
      toast.error('Erreur', { description: 'Impossible de sauvegarder' });
    } finally {
      setIsLoading(false);
    }
  };

  const handleCancel = () => {
    setIsEditing(false);
    setFormData({});
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Settings className="w-6 h-6 text-primary" />
          <h1 className="text-3xl font-bold">Configuration du Score Engine</h1>
        </div>
        {!isEditing && (
          <Button onClick={handleCreateNew} className="flex items-center gap-2">
            <Plus className="w-4 h-4" /> Nouvelle Configuration
          </Button>
        )}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Configurations List */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          className="lg:col-span-1 space-y-3"
        >
          <h2 className="text-lg font-semibold">Configurations Disponibles</h2>

          <div className="space-y-2 max-h-96 overflow-y-auto">
            {configs?.default && (
              <motion.button
                onClick={() => !isEditing && handleEdit(configs.default)}
                className={`w-full text-left p-4 border rounded-lg transition-all ${
                  selectedConfig?.id === 'default'
                    ? 'bg-blue-500/10 border-blue-500'
                    : 'bg-gray-900 border-gray-700 hover:border-gray-600'
                }`}
                disabled={isEditing}
                whileHover={{ scale: 1.02 }}
              >
                <p className="font-medium">{configs.default.name}</p>
                <p className="text-xs text-gray-400 mt-1">
                  {configs.default.description || 'Configuration par défaut'}
                </p>
                {configs.default.is_default && (
                  <span className="inline-block text-xs bg-green-500/20 text-green-400 px-2 py-1 rounded mt-2">
                    Par défaut
                  </span>
                )}
              </motion.button>
            )}
          </div>
        </motion.div>

        {/* Configuration Details */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          className="lg:col-span-2"
        >
          {isEditing ? (
            <div className="bg-gray-900 border border-gray-800 rounded-lg p-6 space-y-6">
              <h2 className="text-xl font-bold">
                {selectedConfig ? 'Modifier Configuration' : 'Créer Configuration'}
              </h2>

              {/* Form Fields */}
              <div className="space-y-4">
                <Input
                  label="ID Configuration"
                  placeholder="ex: custom-config-1"
                  value={formData.id || ''}
                  onChange={(e) => setFormData({ ...formData, id: e.target.value })}
                  disabled={!!selectedConfig || isLoading}
                />

                <Input
                  label="Nom"
                  placeholder="Configuration personnalisée"
                  value={formData.name || ''}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  disabled={isLoading}
                />

                <div className="space-y-2">
                  <label className="text-sm font-medium">Description</label>
                  <textarea
                    value={formData.description || ''}
                    onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                    className="w-full bg-gray-800 border border-gray-700 rounded-lg p-3 text-white text-sm"
                    rows={3}
                    disabled={isLoading}
                  />
                </div>

                <Input
                  label="Formule de Base"
                  placeholder="impact*probability"
                  value={formData.base_formula || ''}
                  onChange={(e) => setFormData({ ...formData, base_formula: e.target.value })}
                  disabled={isLoading}
                />

                {/* Weighting Factors */}
                <div className="border-t border-gray-700 pt-4">
                  <h3 className="font-semibold mb-3">Facteurs de Pondération</h3>
                  <div className="grid grid-cols-2 gap-3">
                    {Object.entries(formData.weighting_factors || {}).map(([key, value]) => (
                      <Input
                        key={key}
                        label={`${key.charAt(0).toUpperCase() + key.slice(1)}`}
                        type="number"
                        step="0.1"
                        value={value.toString()}
                        onChange={(e) =>
                          setFormData({
                            ...formData,
                            weighting_factors: {
                              ...formData.weighting_factors,
                              [key]: parseFloat(e.target.value),
                            },
                          })
                        }
                        disabled={isLoading}
                      />
                    ))}
                  </div>
                </div>

                {/* Risk Matrix Thresholds */}
                <div className="border-t border-gray-700 pt-4">
                  <h3 className="font-semibold mb-3">Seuils de Matrice de Risque</h3>
                  <div className="grid grid-cols-2 gap-3">
                    {Object.entries(formData.risk_matrix_thresholds || {}).map(([key, value]) => (
                      <Input
                        key={key}
                        label={key.charAt(0).toUpperCase() + key.slice(1)}
                        type="number"
                        value={value.toString()}
                        onChange={(e) =>
                          setFormData({
                            ...formData,
                            risk_matrix_thresholds: {
                              ...formData.risk_matrix_thresholds,
                              [key]: parseInt(e.target.value),
                            },
                          })
                        }
                        disabled={isLoading}
                      />
                    ))}
                  </div>
                </div>

                {/* Asset Criticality */}
                <div className="border-t border-gray-700 pt-4">
                  <h3 className="font-semibold mb-3">Multiplicateurs de Criticité</h3>
                  <div className="grid grid-cols-2 gap-3">
                    {Object.entries(formData.asset_criticality_mult || {}).map(([key, value]) => (
                      <Input
                        key={key}
                        label={key.charAt(0).toUpperCase() + key.slice(1)}
                        type="number"
                        step="0.1"
                        value={value.toString()}
                        onChange={(e) =>
                          setFormData({
                            ...formData,
                            asset_criticality_mult: {
                              ...formData.asset_criticality_mult,
                              [key]: parseFloat(e.target.value),
                            },
                          })
                        }
                        disabled={isLoading}
                      />
                    ))}
                  </div>
                </div>
              </div>

              {/* Action Buttons */}
              <div className="flex gap-3 justify-end border-t border-gray-700 pt-4">
                <Button variant="ghost" onClick={handleCancel} disabled={isLoading}>
                  <X className="w-4 h-4 mr-2" /> Annuler
                </Button>
                <Button onClick={handleSave} isLoading={isLoading}>
                  <Save className="w-4 h-4 mr-2" /> Sauvegarder
                </Button>
              </div>
            </div>
          ) : selectedConfig ? (
            <div className="bg-gray-900 border border-gray-800 rounded-lg p-6 space-y-6">
              <div className="flex justify-between items-start">
                <div>
                  <h2 className="text-2xl font-bold">{selectedConfig.name}</h2>
                  <p className="text-gray-400 mt-1">{selectedConfig.description}</p>
                  {selectedConfig.is_default && (
                    <span className="inline-block text-xs bg-green-500/20 text-green-400 px-3 py-1 rounded-full mt-2">
                      Configuration par défaut
                    </span>
                  )}
                </div>
                {!selectedConfig.is_default && (
                  <div className="flex gap-2">
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => handleEdit(selectedConfig)}
                      className="flex items-center gap-2"
                    >
                      <Edit2 className="w-4 h-4" /> Modifier
                    </Button>
                  </div>
                )}
              </div>

              {/* Configuration Details */}
              <div className="grid grid-cols-2 gap-6">
                <div>
                  <h3 className="text-sm font-semibold text-gray-400 mb-3">Formule</h3>
                  <p className="font-mono text-white">{selectedConfig.base_formula}</p>
                </div>

                <div>
                  <h3 className="text-sm font-semibold text-gray-400 mb-3">Pondérations</h3>
                  <div className="space-y-1">
                    {Object.entries(selectedConfig.weighting_factors).map(([key, value]) => (
                      <div key={key} className="flex justify-between text-sm">
                        <span className="text-gray-400">{key}:</span>
                        <span className="font-medium">{value}</span>
                      </div>
                    ))}
                  </div>
                </div>

                <div>
                  <h3 className="text-sm font-semibold text-gray-400 mb-3">Matrice de Risque</h3>
                  <div className="grid grid-cols-2 gap-2">
                    {Object.entries(selectedConfig.risk_matrix_thresholds).map(([key, value]) => (
                      <div key={key} className="bg-gray-800 rounded p-2 text-sm">
                        <p className="text-gray-400 text-xs">{key}</p>
                        <p className="text-lg font-bold">{value}</p>
                      </div>
                    ))}
                  </div>
                </div>

                <div>
                  <h3 className="text-sm font-semibold text-gray-400 mb-3">Criticités</h3>
                  <div className="space-y-1">
                    {Object.entries(selectedConfig.asset_criticality_mult).map(([key, value]) => (
                      <div key={key} className="flex justify-between text-sm">
                        <span className="text-gray-400">{key}:</span>
                        <span className="font-medium">{value}x</span>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          ) : (
            <div className="bg-gray-900 border border-gray-800 rounded-lg p-8 text-center">
              <p className="text-gray-400">Sélectionnez une configuration pour voir les détails</p>
            </div>
          )}
        </motion.div>
      </div>
    </div>
  );
};

export default ScoreEngineConfiguration;
