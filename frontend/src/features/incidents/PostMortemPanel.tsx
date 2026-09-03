// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The structured review.
//
// An incident that closes without a post-mortem teaches nothing, and the same
// incident happens again. The form is deliberately opinionated: it asks WHY
// (root cause) separately from WHAT (summary), it insists on corrective
// actions, and publishing turns each of those actions into a real mitigation
// plan — because a decision that stays in a document is not a decision.

import { useEffect, useState } from 'react';
import { toast } from 'sonner';
import {
  ClipboardList,
  X,
  Plus,
  Trash2,
  CheckCircle2,
  AlertTriangle,
  Lock,
  ArrowRight,
} from 'lucide-react';
import { Btn, Card } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { usePostMortem, usePostMortemMutations } from './useIncidents';
import type { CorrectiveAction, PostMortemTimelineEntry, PostMortemInput } from './incidentService';

const FIELD_LABEL: Record<string, { fr: string; en: string }> = {
  summary: { fr: 'Résumé', en: 'Summary' },
  root_cause: { fr: 'Cause racine', en: 'Root cause' },
  impact: { fr: 'Impact', en: 'Impact' },
  timeline: { fr: 'Chronologie', en: 'Timeline' },
  corrective_actions: { fr: 'Actions correctives', en: 'Corrective actions' },
};

export function PostMortemPanel({
  incidentId,
  onClose,
}: {
  incidentId: number;
  onClose: () => void;
}) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data: view, isLoading } = usePostMortem(incidentId);
  const { save, publish } = usePostMortemMutations(incidentId);

  const [form, setForm] = useState<PostMortemInput | null>(null);

  useEffect(() => {
    if (!view || form) return;
    const pm = view.post_mortem;
    setForm({
      summary: pm.summary ?? '',
      root_cause: pm.root_cause ?? '',
      contributing_factors: pm.contributing_factors ?? '',
      impact: pm.impact ?? '',
      detection: pm.detection ?? '',
      what_went_well: pm.what_went_well ?? '',
      lessons_learned: pm.lessons_learned ?? '',
      timeline: pm.timeline ?? [],
      corrective_actions: pm.corrective_actions ?? [],
    });
  }, [view, form]);

  const published = view?.post_mortem.status === 'published';
  const missing = view?.missing ?? [];

  const set = <K extends keyof PostMortemInput>(k: K, v: PostMortemInput[K]) =>
    setForm((f) => (f ? { ...f, [k]: v } : f));

  const doSave = async () => {
    if (!form) return;
    try {
      await save.mutateAsync(form);
      toast.success(tr('Brouillon enregistré', 'Draft saved'));
    } catch {
      toast.error(tr('Enregistrement impossible', 'Could not save'));
    }
  };

  const doPublish = async () => {
    if (!form) return;
    try {
      await save.mutateAsync(form);
      const res = await publish.mutateAsync();
      // Report what actually happened — including what could not be converted.
      if (res.mitigations_created > 0) {
        toast.success(
          tr(
            `Publié. ${res.mitigations_created} action(s) corrective(s) sont devenues des plans de mitigation suivis.`,
            `Published. ${res.mitigations_created} corrective action(s) became tracked mitigation plans.`,
          ),
        );
      } else {
        toast.success(tr('Post-mortem publié', 'Post-mortem published'));
      }
      (res.not_converted ?? []).forEach((r) => toast.warning(r));
    } catch (e) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      toast.error(msg || tr('Publication impossible', 'Could not publish'));
    }
  };

  const field = 'w-full px-3 py-2 rounded-[9px] text-[13px] outline-none';
  const fieldStyle = {
    border: '1px solid var(--border-strong)',
    background: 'var(--bg)',
    color: 'var(--fg-primary)',
  } as const;

  const Text = ({
    k,
    label,
    hint,
    rows = 3,
  }: {
    k: keyof PostMortemInput;
    label: string;
    hint: string;
    rows?: number;
  }) => (
    <label className="block">
      <span className="text-[12px] font-semibold" style={{ color: 'var(--fg-secondary)' }}>
        {label}
      </span>
      <span className="block text-[11.5px] mb-1" style={{ color: 'var(--fg-secondary)' }}>
        {hint}
      </span>
      <textarea
        rows={rows}
        className={field}
        style={fieldStyle}
        disabled={published}
        value={(form?.[k] as string) ?? ''}
        onChange={(e) => set(k, e.target.value as never)}
      />
    </label>
  );

  const addTimelineEntry = () =>
    set('timeline', [
      ...(form?.timeline ?? []),
      { at: new Date().toISOString(), title: '', kind: 'note' } as PostMortemTimelineEntry,
    ]);

  const addAction = () =>
    set('corrective_actions', [
      ...(form?.corrective_actions ?? []),
      { id: '', title: '', priority: 'medium', status: 'open' } as CorrectiveAction,
    ]);

  return (
    <div className="fixed inset-0 z-50 flex justify-end" style={{ background: 'rgba(0,0,0,.35)' }}>
      <div
        className="w-full max-w-[760px] h-full overflow-y-auto or-slidein"
        style={{ background: 'var(--bg-elevated)', borderLeft: '1px solid var(--border-strong)' }}
      >
        <div
          className="sticky top-0 z-10 flex items-center justify-between px-5 py-4"
          style={{ background: 'var(--bg-elevated)', borderBottom: '1px solid var(--border)' }}
        >
          <div>
            <h2
              className="text-[15px] font-bold inline-flex items-center gap-2"
              style={{ color: 'var(--fg-primary)' }}
            >
              <ClipboardList size={16} style={{ color: 'var(--accent-500)' }} />
              {tr('Post-mortem', 'Post-mortem')} — INC-{incidentId}
            </h2>
            <p className="text-[12px] mt-0.5" style={{ color: 'var(--fg-secondary)' }}>
              {published
                ? tr('Publié — le compte rendu est figé.', 'Published — the record is frozen.')
                : tr(
                    'Brouillon. Publier fige le compte rendu et crée les plans de mitigation.',
                    'Draft. Publishing freezes the record and creates the mitigation plans.',
                  )}
            </p>
          </div>
          <button onClick={onClose} className="p-1.5 rounded-lg" aria-label={tr('Fermer', 'Close')}>
            <X size={18} />
          </button>
        </div>

        <div className="p-5 space-y-4">
          {isLoading && (
            <p className="text-[12.5px]" style={{ color: 'var(--fg-secondary)' }}>
              {tr('Chargement…', 'Loading…')}
            </p>
          )}

          {view?.blocks_closure && (
            <div
              className="rounded-[10px] p-3 flex items-start gap-2.5 text-[12.5px]"
              style={{
                background: 'color-mix(in srgb, var(--critical) 8%, transparent)',
                border: '1px solid color-mix(in srgb, var(--critical) 35%, transparent)',
                color: 'var(--fg-primary)',
              }}
            >
              <AlertTriangle
                size={15}
                style={{ color: 'var(--critical)' }}
                className="shrink-0 mt-0.5"
              />
              <span>{view.blocks_closure}</span>
            </div>
          )}

          {published && (
            <div
              className="rounded-[10px] p-3 flex items-start gap-2.5 text-[12.5px]"
              style={{ background: 'var(--bg-hover)', color: 'var(--fg-secondary)' }}
            >
              <Lock size={15} className="shrink-0 mt-0.5" />
              <span>
                {tr(
                  'Un post-mortem publié n’est plus modifiable : c’est le compte rendu de ce que l’organisation a conclu.',
                  'A published post-mortem can no longer be edited: it is the record of what the organisation concluded.',
                )}
              </span>
            </div>
          )}

          {form && (
            <>
              <Text
                k="summary"
                label={tr('Résumé', 'Summary')}
                hint={tr(
                  'Ce qui s’est passé, en trois phrases, pour quelqu’un qui n’était pas là.',
                  'What happened, in three sentences, for somebody who was not there.',
                )}
              />
              <Text
                k="root_cause"
                label={tr('Cause racine', 'Root cause')}
                hint={tr(
                  'Pourquoi c’est arrivé — pas ce qui est arrivé. Descendez jusqu’à la décision ou l’absence de décision.',
                  'Why it happened — not what happened. Go down to the decision, or the missing one.',
                )}
              />
              <Text
                k="contributing_factors"
                label={tr('Facteurs aggravants', 'Contributing factors')}
                hint={tr(
                  'Les conditions qui ont permis à la cause racine de mordre.',
                  'The conditions that let the root cause bite.',
                )}
                rows={2}
              />
              <Text
                k="impact"
                label={tr('Impact', 'Impact')}
                hint={tr(
                  'Ce que ça a coûté : utilisateurs, données, indisponibilité, argent, obligations.',
                  'What it cost: users, data, downtime, money, obligations.',
                )}
                rows={2}
              />
              <Text
                k="detection"
                label={tr('Détection', 'Detection')}
                hint={tr(
                  'Comment on l’a su. Y compris « un client nous l’a dit » — c’est le constat qui change les feuilles de route.',
                  'How we found out. Including "a customer told us" — that is the finding that changes roadmaps.',
                )}
                rows={2}
              />
              <Text
                k="what_went_well"
                label={tr('Ce qui a bien fonctionné', 'What went well')}
                hint={tr(
                  'Une revue qui ne liste que des échecs apprend aux gens à cacher les incidents.',
                  'A review that lists only failures teaches people to hide incidents.',
                )}
                rows={2}
              />

              {/* Timeline */}
              <div>
                <div className="flex items-center justify-between">
                  <span
                    className="text-[12px] font-semibold"
                    style={{ color: 'var(--fg-secondary)' }}
                  >
                    {tr('Chronologie', 'Timeline')}
                  </span>
                  {!published && (
                    <button
                      onClick={addTimelineEntry}
                      className="text-[12px] font-semibold inline-flex items-center gap-1"
                      style={{ color: 'var(--accent-500)' }}
                    >
                      <Plus size={13} /> {tr('Ajouter un moment', 'Add a moment')}
                    </button>
                  )}
                </div>
                <div className="mt-1.5 space-y-2">
                  {(form.timeline ?? []).map((e, i) => (
                    <div key={i} className="flex gap-2">
                      <input
                        type="datetime-local"
                        className={field}
                        style={{ ...fieldStyle, maxWidth: 210 }}
                        disabled={published}
                        value={e.at ? new Date(e.at).toISOString().slice(0, 16) : ''}
                        onChange={(ev) =>
                          set(
                            'timeline',
                            form.timeline.map((x, j) =>
                              j === i ? { ...x, at: new Date(ev.target.value).toISOString() } : x,
                            ),
                          )
                        }
                      />
                      <input
                        className={field}
                        style={fieldStyle}
                        disabled={published}
                        placeholder={tr('Que s’est-il passé ?', 'What happened?')}
                        value={e.title}
                        onChange={(ev) =>
                          set(
                            'timeline',
                            form.timeline.map((x, j) =>
                              j === i ? { ...x, title: ev.target.value } : x,
                            ),
                          )
                        }
                      />
                      {!published && (
                        <button
                          onClick={() =>
                            set(
                              'timeline',
                              form.timeline.filter((_, j) => j !== i),
                            )
                          }
                          className="w-9 h-9 rounded-[9px] flex items-center justify-center shrink-0"
                          style={{ background: 'var(--bg-hover)', color: 'var(--critical)' }}
                        >
                          <Trash2 size={14} />
                        </button>
                      )}
                    </div>
                  ))}
                  {(form.timeline ?? []).length === 0 && (
                    <p className="text-[12px]" style={{ color: 'var(--fg-secondary)' }}>
                      {tr(
                        'Reconstituez la séquence : détection, escalade, atténuation, résolution.',
                        'Reconstruct the sequence: detection, escalation, mitigation, resolution.',
                      )}
                    </p>
                  )}
                </div>
              </div>

              {/* Corrective actions — the part that leaves the document. */}
              <div>
                <div className="flex items-center justify-between">
                  <span
                    className="text-[12px] font-semibold"
                    style={{ color: 'var(--fg-secondary)' }}
                  >
                    {tr('Actions correctives', 'Corrective actions')}
                  </span>
                  {!published && (
                    <button
                      onClick={addAction}
                      className="text-[12px] font-semibold inline-flex items-center gap-1"
                      style={{ color: 'var(--accent-500)' }}
                    >
                      <Plus size={13} /> {tr('Ajouter', 'Add')}
                    </button>
                  )}
                </div>
                <p className="text-[11.5px] mb-1.5" style={{ color: 'var(--fg-secondary)' }}>
                  {tr(
                    'À la publication, chacune devient un plan de mitigation réel, suivi dans le module Mitigations.',
                    'On publication, each becomes a real mitigation plan, tracked in the Mitigations module.',
                  )}
                </p>
                <div className="space-y-2">
                  {(form.corrective_actions ?? []).map((a, i) => (
                    <Card key={i} style={{ padding: '10px 12px' }}>
                      <div className="flex gap-2 items-start">
                        <div className="flex-1 space-y-2">
                          <input
                            className={field}
                            style={fieldStyle}
                            disabled={published}
                            placeholder={tr('Ce qu’il faut faire', 'What has to be done')}
                            value={a.title}
                            onChange={(ev) =>
                              set(
                                'corrective_actions',
                                form.corrective_actions.map((x, j) =>
                                  j === i ? { ...x, title: ev.target.value } : x,
                                ),
                              )
                            }
                          />
                          <div className="flex gap-2">
                            <select
                              className={field}
                              style={{ ...fieldStyle, maxWidth: 150 }}
                              disabled={published}
                              value={a.priority ?? 'medium'}
                              onChange={(ev) =>
                                set(
                                  'corrective_actions',
                                  form.corrective_actions.map((x, j) =>
                                    j === i ? { ...x, priority: ev.target.value } : x,
                                  ),
                                )
                              }
                            >
                              <option value="critical">critical</option>
                              <option value="high">high</option>
                              <option value="medium">medium</option>
                              <option value="low">low</option>
                            </select>
                            <input
                              type="date"
                              className={field}
                              style={{ ...fieldStyle, maxWidth: 170 }}
                              disabled={published}
                              value={a.due_date ? a.due_date.slice(0, 10) : ''}
                              onChange={(ev) =>
                                set(
                                  'corrective_actions',
                                  form.corrective_actions.map((x, j) =>
                                    j === i
                                      ? {
                                          ...x,
                                          due_date: ev.target.value
                                            ? new Date(ev.target.value).toISOString()
                                            : null,
                                        }
                                      : x,
                                  ),
                                )
                              }
                            />
                          </div>
                          {a.mitigation_id && (
                            <p
                              className="text-[11.5px] inline-flex items-center gap-1"
                              style={{ color: 'var(--low)' }}
                            >
                              <CheckCircle2 size={12} />
                              {tr('Suivi comme plan de mitigation', 'Tracked as a mitigation plan')}
                              <a
                                href={`/mitigations?focus=${a.mitigation_id}`}
                                className="inline-flex items-center gap-0.5 font-semibold"
                                style={{ color: 'var(--accent-500)' }}
                              >
                                {tr('ouvrir', 'open')} <ArrowRight size={11} />
                              </a>
                            </p>
                          )}
                        </div>
                        {!published && (
                          <button
                            onClick={() =>
                              set(
                                'corrective_actions',
                                form.corrective_actions.filter((_, j) => j !== i),
                              )
                            }
                            className="w-9 h-9 rounded-[9px] flex items-center justify-center shrink-0"
                            style={{ background: 'var(--bg-hover)', color: 'var(--critical)' }}
                          >
                            <Trash2 size={14} />
                          </button>
                        )}
                      </div>
                    </Card>
                  ))}
                  {(form.corrective_actions ?? []).length === 0 && (
                    <p className="text-[12px]" style={{ color: 'var(--medium)' }}>
                      {tr(
                        'Une revue sans action corrective est un récit, pas une revue.',
                        'A review with no corrective action is a story, not a review.',
                      )}
                    </p>
                  )}
                </div>
              </div>

              {/* The checklist — a reviewer sees the remaining fields, not a wall. */}
              {!published && missing.length > 0 && (
                <div
                  className="rounded-[10px] p-3 text-[12.5px]"
                  style={{ background: 'var(--bg-hover)', color: 'var(--fg-secondary)' }}
                >
                  {tr(
                    'Reste à remplir avant publication : ',
                    'Still to fill in before publishing: ',
                  )}
                  <strong style={{ color: 'var(--fg-primary)' }}>
                    {missing
                      .map((m) => (lang === 'fr' ? FIELD_LABEL[m]?.fr : FIELD_LABEL[m]?.en) ?? m)
                      .join(' · ')}
                  </strong>
                </div>
              )}

              {!published && (
                <div className="flex justify-end gap-2 pt-1">
                  <Btn
                    label={tr('Enregistrer le brouillon', 'Save draft')}
                    onClick={doSave}
                    disabled={save.isPending}
                  />
                  <Btn
                    label={tr('Publier', 'Publish')}
                    icon={CheckCircle2}
                    primary
                    onClick={doPublish}
                    disabled={publish.isPending || missing.length > 0}
                  />
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  );
}
