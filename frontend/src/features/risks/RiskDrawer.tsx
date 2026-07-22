// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useMemo, useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { X, Share2, Copy, CheckCircle2, Pencil, Trash2, ChevronRight, Plus } from 'lucide-react';
import { RiskBadge, ScoreMeter, StatusDot, UserAvatar } from '../../components/shared';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Drawer } from '../../components/ui/Drawer';
import { useI18n } from '../../hooks/useI18n';
import { riskService, type Risk } from '../../services/riskService';

interface RiskDrawerProps {
  risk: Risk | null;
  isOpen: boolean;
  onClose: () => void;
  onDelete: (id: string) => Promise<void>;
  onDuplicate: (id: string) => Promise<void>;
  onAccept: (id: string, justification: string) => Promise<void>;
  onUpdate: (id: string, payload: Partial<Risk>) => Promise<void>;
}

type DrawerTab = 'details' | 'score' | 'mitigations' | 'timeline' | 'cti' | 'ai' | 'financial';

const tabs: Array<{ id: DrawerTab; label: string }> = [
  { id: 'details', label: 'Détails' },
  { id: 'score', label: 'Score' },
  { id: 'mitigations', label: 'Mitigations' },
  { id: 'timeline', label: 'Timeline' },
  { id: 'cti', label: 'CTI' },
  { id: 'ai', label: 'IA' },
  { id: 'financial', label: 'Financier' },
];

const getLevelFromScore = (score: number) => {
  if (score >= 80) return 'CRITICAL' as const;
  if (score >= 60) return 'HIGH' as const;
  if (score >= 40) return 'MEDIUM' as const;
  return 'LOW' as const;
};

interface DynamicFieldProps {
  label: string;
  value: string | number;
  type?: 'text' | 'textarea' | 'number';
  onChange: (value: string) => void;
  onBlur?: () => void;
  description?: string;
  rows?: number;
}

const DynamicField = ({ label, value, type = 'text', onChange, onBlur, description, rows = 3 }: DynamicFieldProps) => (
  <div className="space-y-2">
    <label className="text-xs font-semibold uppercase tracking-widest text-zinc-500">{label}</label>
    {type === 'textarea' ? (
      <textarea
        value={value as string}
        onChange={(event) => onChange(event.target.value)}
        onBlur={onBlur}
        rows={rows}
        className="w-full min-h-[100px] rounded-2xl border border-zinc-800 bg-zinc-950 px-4 py-3 text-sm text-white focus:outline-none focus:ring-2 focus:ring-primary/40"
      />
    ) : (
      <Input
        value={String(value)}
        onChange={(event) => onChange(event.target.value)}
        onBlur={onBlur}
        type={type}
        className="bg-zinc-950"
      />
    )}
    {description ? <p className="text-xs text-zinc-500">{description}</p> : null}
  </div>
);

const getStatusLabel = (status: Risk['status']) => {
  switch (status) {
    case 'open': return 'Ouvert';
    case 'in_progress': return 'En cours';
    case 'mitigated': return 'Atténué';
    case 'accepted': return 'Accepté';
    case 'closed': return 'Fermé';
    default: return status;
  }
};

export const RiskDrawer = ({ risk, isOpen, onClose, onDelete, onDuplicate, onAccept, onUpdate }: RiskDrawerProps) => {
  const { t } = useI18n();
  const [activeTab, setActiveTab] = useState<DrawerTab>('details');
  const [localTitle, setLocalTitle] = useState('');
  const [localDescription, setLocalDescription] = useState('');
  const [localImpact, setLocalImpact] = useState(0);
  const [localProbability, setLocalProbability] = useState(0);
  const [assetCriticality, setAssetCriticality] = useState(1.5);
  const [acceptReason, setAcceptReason] = useState('');
  const [isSaving, setIsSaving] = useState(false);
  const [localSLE, setLocalSLE] = useState('');
  const [localARO, setLocalARO] = useState('');
  const [savingCRQ, setSavingCRQ] = useState(false);
  const [reviewInterval, setReviewInterval] = useState(0);
  const [nextReview, setNextReview] = useState<string | null>(null);
  const [lastReviewed, setLastReviewed] = useState<string | null>(null);
  const [reviewing, setReviewing] = useState(false);

  useEffect(() => {
    if (!risk) return;
    setLocalTitle(risk.title);
    setLocalDescription(risk.description);
    setLocalImpact(risk.impact);
    setLocalProbability(risk.probability);
    setAssetCriticality(risk.assets?.[0]?.criticality === 'CRITICAL' ? 3 : risk.assets?.[0]?.criticality === 'HIGH' ? 2 : risk.assets?.[0]?.criticality === 'MEDIUM' ? 1.5 : 1) ;
    setAcceptReason('');
    setLocalSLE(risk.sle_xaf != null ? String(risk.sle_xaf) : '');
    setLocalARO(risk.aro != null ? String(risk.aro) : '');
    setReviewInterval(risk.review_interval_days ?? 0);
    setNextReview(risk.next_review_at ?? null);
    setLastReviewed(risk.last_reviewed_at ?? null);
  }, [risk]);

  const reviewOverdue = nextReview != null && new Date(nextReview).getTime() < Date.now();

  const handleSaveInterval = async (days: number) => {
    if (!risk) return;
    setReviewInterval(days);
    await onUpdate(risk.id, { review_interval_days: days });
  };

  const handleMarkReviewed = async () => {
    if (!risk) return;
    setReviewing(true);
    try {
      const updated = await riskService.markReviewed(risk.id);
      setNextReview(updated.next_review_at ?? null);
      setLastReviewed(updated.last_reviewed_at ?? null);
    } finally {
      setReviewing(false);
    }
  };

  const fmtXAF = (v?: number) => (v == null ? '—' : `${Math.round(v).toLocaleString('fr-FR')} FCFA`);
  const fmtUSD = (v?: number) => (v == null ? '—' : `$${v.toLocaleString('en-US', { maximumFractionDigits: 0 })}`);

  const handleSaveCRQ = async () => {
    if (!risk) return;
    setSavingCRQ(true);
    try {
      await onUpdate(risk.id, {
        sle_xaf: localSLE.trim() === '' ? null : Number(localSLE),
        aro: localARO.trim() === '' ? null : Number(localARO),
      });
    } finally {
      setSavingCRQ(false);
    }
  };

  const score = useMemo(() => {
    const normalizedImpact = Math.min(10, Math.max(0, localImpact));
    const normalizedProbability = Math.min(1, Math.max(0, localProbability));
    return Number((normalizedProbability * normalizedImpact * assetCriticality).toFixed(1));
  }, [localImpact, localProbability, assetCriticality]);

  if (!risk) return null;

  const handleSaveInline = async () => {
    if (!risk) return;
    setIsSaving(true);
    try {
      await onUpdate(risk.id, {
        title: localTitle,
        description: localDescription,
        impact: localImpact,
        probability: localProbability,
      });
    } finally {
      setIsSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!risk) return;
    if (!confirm(t('risks.deleteConfirm'))) return;
    await onDelete(risk.id);
    onClose();
  };

  const handleDuplicate = async () => {
    if (!risk) return;
    await onDuplicate(risk.id);
  };

  const handleAccept = async () => {
    if (!risk) return;
    await onAccept(risk.id, acceptReason);
  };

  return (
    <Drawer isOpen={isOpen} onClose={onClose} title={risk.title} widthClassName="max-w-[560px]">
      <div className="space-y-6">
        <div className="sticky top-0 z-10 bg-surface/95 backdrop-blur-md border-b border-zinc-800 pb-4">
          <div className="flex items-center justify-between gap-4">
            <div className="space-y-3">
              <div className="flex flex-wrap items-center gap-3">
                <RiskBadge level={risk.level ?? getLevelFromScore(risk.score)} size="md" />
                <ScoreMeter score={risk.score} maxScore={100} size="sm" showLabel={false} />
              </div>
              <div className="flex items-center gap-2 text-xs text-zinc-400">
                <StatusDot status={risk.status ?? 'open'} size="xs" withLabel />
                <span>{t('risks.riskUpdatedAt')}: {risk.updated_at ? new Date(risk.updated_at).toLocaleString() : '-'}</span>
              </div>
            </div>
            <button onClick={onClose} className="p-2 rounded-full hover:bg-white/10 transition-colors text-zinc-400">
              <X size={18} />
            </button>
          </div>

          <div className="mt-4 grid grid-cols-2 gap-2">
            <Button onClick={handleSaveInline} variant="secondary" isLoading={isSaving}>
              <Pencil size={16} /> Sauvegarder
            </Button>
            <Button onClick={handleDuplicate} variant="ghost" className="gap-2">
              <Copy size={16} /> Dupliquer
            </Button>
          </div>
        </div>

        <div className="overflow-x-auto">
          <div className="inline-flex rounded-full border border-zinc-800 bg-zinc-950 p-1">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                type="button"
                onClick={() => setActiveTab(tab.id)}
                className={`rounded-full px-4 py-2 text-sm font-semibold transition-all ${activeTab === tab.id ? 'bg-primary text-black' : 'text-zinc-400 hover:text-white'}`}
              >
                {tab.label}
              </button>
            ))}
          </div>
        </div>

        <div className="space-y-6">
          {activeTab === 'details' && (
            <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }}>
              <div className="grid gap-4">
                <DynamicField
                  label="Titre"
                  value={localTitle}
                  onChange={setLocalTitle}
                  onBlur={handleSaveInline}
                />
                <DynamicField
                  label="Description"
                  value={localDescription}
                  type="textarea"
                  onChange={setLocalDescription}
                  onBlur={handleSaveInline}
                />
                <div className="grid grid-cols-2 gap-4">
                  <DynamicField
                    label="Impact"
                    value={localImpact}
                    type="number"
                    onChange={(value) => setLocalImpact(Number(value))}
                    onBlur={handleSaveInline}
                  />
                  <DynamicField
                    label="Probabilité"
                    value={localProbability}
                    type="number"
                    onChange={(value) => setLocalProbability(Number(value))}
                    onBlur={handleSaveInline}
                  />
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <label className="text-xs font-semibold uppercase tracking-widest text-zinc-500">Criticité de l'asset</label>
                    <input
                      type="range"
                      min={0.1}
                      max={3}
                      step={0.1}
                      value={assetCriticality}
                      onChange={(event) => setAssetCriticality(Number(event.target.value))}
                      className="w-full"
                    />
                    <div className="text-xs text-zinc-400">{assetCriticality.toFixed(1)}</div>
                  </div>
                  <div className="space-y-2">
                    <label className="text-xs font-semibold uppercase tracking-widest text-zinc-500">Score local</label>
                    <div className="rounded-2xl border border-zinc-800 bg-zinc-950 p-4 text-center">
                      <div className="text-3xl font-bold text-white">{score}</div>
                      <div className="text-xs text-zinc-500">P × I × A</div>
                    </div>
                  </div>
                </div>

                {/* Review cadence */}
                <div className="rounded-3xl border border-zinc-800 bg-zinc-950 p-4 space-y-3">
                  <div className="flex items-center justify-between gap-2">
                    <label className="text-xs font-semibold uppercase tracking-widest text-zinc-500">Cadence de revue</label>
                    {reviewOverdue && (
                      <span className="text-[11px] font-semibold rounded-full px-2 py-0.5" style={{ color: '#f87171', background: 'rgba(248,113,113,0.12)' }}>
                        En retard
                      </span>
                    )}
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <select
                      value={reviewInterval}
                      onChange={(e) => handleSaveInterval(Number(e.target.value))}
                      className="w-full rounded-2xl border border-zinc-800 bg-zinc-900 px-4 py-2.5 text-sm text-white focus:outline-none focus:ring-2 focus:ring-primary/40"
                    >
                      <option value={0}>Manuel</option>
                      <option value={7}>Hebdomadaire</option>
                      <option value={30}>Mensuel</option>
                      <option value={90}>Trimestriel</option>
                    </select>
                    <Button onClick={handleMarkReviewed} variant="secondary" isLoading={reviewing} className="gap-2">
                      <CheckCircle2 size={16} /> Marquer revu
                    </Button>
                  </div>
                  <div className="grid grid-cols-2 gap-3 text-xs text-zinc-500">
                    <div>Prochaine : <span className="text-zinc-300">{nextReview ? new Date(nextReview).toLocaleDateString() : '—'}</span></div>
                    <div>Dernière : <span className="text-zinc-300">{lastReviewed ? new Date(lastReviewed).toLocaleDateString() : '—'}</span></div>
                  </div>
                </div>
              </div>
            </motion.div>
          )}

          {activeTab === 'score' && (
            <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }}>
              <div className="grid gap-6">
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <p className="text-xs text-zinc-500 uppercase tracking-widest">Score</p>
                    <h2 className="text-3xl font-bold text-white">{risk.score.toFixed(1)}</h2>
                  </div>
                  <RiskBadge level={risk.level ?? getLevelFromScore(risk.score)} />
                </div>
                <div className="rounded-3xl border border-zinc-800 bg-zinc-950 p-5 space-y-5">
                  <div className="grid gap-4 sm:grid-cols-3">
                    <div className="rounded-2xl border border-zinc-800 p-4 bg-zinc-900/40">
                      <p className="text-xs text-zinc-500 uppercase">Probabilité</p>
                      <p className="text-2xl font-semibold text-white">{risk.probability}</p>
                    </div>
                    <div className="rounded-2xl border border-zinc-800 p-4 bg-zinc-900/40">
                      <p className="text-xs text-zinc-500 uppercase">Impact</p>
                      <p className="text-2xl font-semibold text-white">{risk.impact}</p>
                    </div>
                    <div className="rounded-2xl border border-zinc-800 p-4 bg-zinc-900/40">
                      <p className="text-xs text-zinc-500 uppercase">Asset</p>
                      <p className="text-2xl font-semibold text-white">{assetCriticality.toFixed(1)}</p>
                    </div>
                  </div>
                  <div className="rounded-3xl border border-zinc-800 bg-zinc-900/40 p-4">
                    <p className="text-xs text-zinc-500 uppercase">Détail du calcul</p>
                    <div className="mt-4 grid gap-3 text-sm text-zinc-300">
                      <div className="flex justify-between"><span>Probability</span><span>{risk.probability}</span></div>
                      <div className="flex justify-between"><span>Impact</span><span>{risk.impact}</span></div>
                      <div className="flex justify-between"><span>Asset Criticality</span><span>{assetCriticality.toFixed(1)}</span></div>
                      <div className="flex justify-between font-semibold text-white"><span>Total</span><span>{score}</span></div>
                    </div>
                  </div>
                </div>
              </div>
            </motion.div>
          )}

          {activeTab === 'mitigations' && (
            <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }}>
              <div className="space-y-4">
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <h3 className="text-lg font-semibold text-white">Plans d'atténuation</h3>
                    <p className="text-sm text-zinc-500">Progression globale et actions disponibles.</p>
                  </div>
                  <Button variant="secondary" className="gap-2"><Plus size={16} /> Nouveau plan</Button>
                </div>
                {risk.mitigations?.length ? (
                  <div className="space-y-3">
                    {risk.mitigations.map((mitigation) => (
                      <div key={mitigation.id} className="rounded-3xl border border-zinc-800 bg-zinc-900/60 p-4">
                        <div className="flex items-start justify-between gap-4">
                          <div>
                            <h4 className="font-semibold text-white">{mitigation.title}</h4>
                            <p className="text-xs text-zinc-500">{mitigation.assignee ?? 'Non assigné'}</p>
                          </div>
                          <span className="text-xs rounded-full bg-zinc-800 px-2 py-1 text-zinc-300">{mitigation.status}</span>
                        </div>
                        <div className="mt-3 h-2 rounded-full bg-zinc-800 overflow-hidden">
                          <div className="h-full bg-primary" style={{ width: `${mitigation.progress}%` }} />
                        </div>
                        <div className="mt-2 flex justify-between text-[11px] text-zinc-500"><span>Progression</span><span>{mitigation.progress}%</span></div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="rounded-3xl border border-dashed border-zinc-700 bg-zinc-900/50 p-6 text-center text-zinc-400">
                    Aucune mitigation définie pour l'instant.
                  </div>
                )}
              </div>
            </motion.div>
          )}

          {activeTab === 'timeline' && (
            <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }}>
              <div className="space-y-4">
                <h3 className="text-lg font-semibold text-white">Chronologie</h3>
                <div className="space-y-3">
                  {[
                    { title: 'Risque créé', description: 'Création initiale du risque.', by: 'Admin', date: risk.created_at || '' },
                    { title: 'Statut mis à jour', description: 'Le risque est passé à En cours.', by: 'Sara', date: risk.updated_at || '' },
                  ].map((entry, index) => (
                    <div key={index} className="flex gap-4 rounded-3xl border border-zinc-800 bg-zinc-900/60 p-4">
                      <div className="flex h-10 w-10 items-center justify-center rounded-full bg-zinc-800 text-zinc-400">{index + 1}</div>
                      <div className="min-w-0">
                        <div className="flex items-center justify-between gap-2">
                          <p className="text-sm font-semibold text-white">{entry.title}</p>
                          <span className="text-[11px] text-zinc-500">{entry.date ? new Date(entry.date).toLocaleDateString() : '-'}</span>
                        </div>
                        <p className="text-xs text-zinc-500 mt-1">{entry.description}</p>
                        <p className="text-[11px] text-zinc-500 mt-2">Par {entry.by}</p>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </motion.div>
          )}

          {activeTab === 'cti' && (
            <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }}>
              <div className="space-y-4">
                <h3 className="text-lg font-semibold text-white">CTI</h3>
                <div className="rounded-3xl border border-zinc-800 bg-zinc-900/60 p-4 space-y-4">
                  <div className="flex items-center justify-between gap-4">
                    <div>
                      <p className="text-sm font-semibold text-white">CVE liées</p>
                      <p className="text-xs text-zinc-500">Extrait du flux de menace.</p>
                    </div>
                    <span className="text-xs rounded-full bg-zinc-800 px-2 py-1 text-zinc-400">2 CVEs</span>
                  </div>
                  <div className="grid gap-3">
                    {['CVE-2025-1234', 'CVE-2024-9876'].map((cve) => (
                      <div key={cve} className="rounded-3xl border border-zinc-800 bg-zinc-950 p-4">
                        <div className="flex items-center justify-between gap-2">
                          <p className="font-semibold text-white">{cve}</p>
                          <button className="text-xs text-primary hover:text-primary/80">Voir</button>
                        </div>
                        <p className="text-xs text-zinc-500 mt-2">Mapping MITRE: TA0001, T1190</p>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            </motion.div>
          )}

          {activeTab === 'ai' && (
            <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }}>
              <div className="rounded-3xl border border-zinc-800 bg-zinc-900/60 p-5 space-y-4">
                <div className="flex items-center justify-between gap-2">
                  <div>
                    <h3 className="text-lg font-semibold text-white">AI Advisor</h3>
                    <p className="text-sm text-zinc-500">Suggestions d'amélioration et actions priorisées.</p>
                  </div>
                  <span className="text-xs rounded-full bg-primary/10 px-2 py-1 text-primary">IA</span>
                </div>
                <div className="space-y-3">
                  <div className="rounded-3xl border border-zinc-800 bg-zinc-950 p-4">
                    <p className="text-sm font-semibold text-white">Suggestion</p>
                    <p className="text-sm text-zinc-300 mt-2">Augmenter la surveillance réseau et appliquer un patch prioritaire sur les assets exposés.</p>
                  </div>
                  <div className="rounded-3xl border border-zinc-800 bg-zinc-950 p-4">
                    <p className="text-sm font-semibold text-white">Confiance</p>
                    <p className="text-sm text-zinc-300 mt-2">L'analyse IA est basée sur le score de risque, l'impact et le contexte CTI.</p>
                  </div>
                </div>
              </div>
            </motion.div>
          )}

          {activeTab === 'financial' && (
            <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }}>
              <div className="space-y-4">
                <div>
                  <h3 className="text-lg font-semibold text-white">Quantification du risque (CRQ)</h3>
                  <p className="text-sm text-zinc-500">Perte annuelle attendue (ALE = SLE × ARO) en FCFA et USD.</p>
                </div>

                {/* Computed ALE */}
                <div className="grid grid-cols-2 gap-3">
                  <div className="rounded-3xl border border-primary/30 bg-primary/5 p-4">
                    <p className="text-[11px] uppercase tracking-widest text-zinc-500">ALE (FCFA)</p>
                    <p className="text-2xl font-bold text-white mt-1">{fmtXAF(risk.ale_xaf)}</p>
                  </div>
                  <div className="rounded-3xl border border-zinc-800 bg-zinc-900/60 p-4">
                    <p className="text-[11px] uppercase tracking-widest text-zinc-500">ALE (USD)</p>
                    <p className="text-2xl font-bold text-white mt-1">{fmtUSD(risk.ale_usd)}</p>
                  </div>
                </div>
                <div className="text-xs text-zinc-500">
                  Base :{' '}
                  {risk.ale_basis === 'explicit'
                    ? 'saisie explicite (SLE × ARO)'
                    : 'valeur de référence par criticité (aucune saisie)'}
                </div>

                {/* Inputs */}
                <div className="rounded-3xl border border-zinc-800 bg-zinc-950 p-4 space-y-4">
                  <div className="grid grid-cols-2 gap-4">
                    <DynamicField
                      label="SLE — Perte par sinistre (FCFA)"
                      value={localSLE}
                      type="number"
                      onChange={setLocalSLE}
                      description="Coût d'une seule occurrence"
                    />
                    <DynamicField
                      label="ARO — Fréquence annuelle"
                      value={localARO}
                      type="number"
                      onChange={setLocalARO}
                      description="Ex. 0.5 = tous les 2 ans"
                    />
                  </div>
                  <Button onClick={handleSaveCRQ} variant="secondary" isLoading={savingCRQ} className="w-full gap-2">
                    <CheckCircle2 size={16} /> Recalculer l'exposition
                  </Button>
                  <p className="text-[11px] text-zinc-500">
                    Laissez vides pour utiliser la valeur de référence par criticité.
                  </p>
                </div>
              </div>
            </motion.div>
          )}
        </div>

        <div className="flex flex-wrap gap-3 justify-end pt-4 border-t border-zinc-800">
          <Button variant="ghost" onClick={() => navigator.clipboard.writeText(window.location.href)} className="gap-2">
            <Share2 size={16} /> Partager
          </Button>
          <Button variant="ghost" onClick={handleDuplicate} className="gap-2">
            <Copy size={16} /> Dupliquer
          </Button>
          <Button variant="secondary" onClick={handleAccept} className="gap-2">
            <CheckCircle2 size={16} /> Accepter
          </Button>
          <Button variant="danger" onClick={handleDelete} className="gap-2">
            <Trash2 size={16} /> Supprimer
          </Button>
        </div>

        <div className="rounded-3xl border border-zinc-800 bg-zinc-950 p-4">
          <label className="text-xs font-semibold uppercase tracking-widest text-zinc-500 mb-2 block">Justification d'acceptation</label>
          <textarea
            value={acceptReason}
            onChange={(event) => setAcceptReason(event.target.value)}
            rows={3}
            className="w-full rounded-2xl border border-zinc-800 bg-zinc-900 px-4 py-3 text-sm text-white focus:outline-none focus:ring-2 focus:ring-primary/40"
          />
        </div>
      </div>
    </Drawer>
  );
};
