import { useState } from 'react';
import { Shield, Plus, Edit2, Trash2, Loader2 } from 'lucide-react';
import { motion } from 'framer-motion';
import { Card } from '../../../components/Card';
import { Button } from '../../../components/ui/Button';
import { useRiskReview } from '../../../hooks/useRiskManagement';
import { toast } from 'sonner';

interface RiskReview {
  id: string;
  riskTitle: string;
  reviewType: string;
  reviewDate: string;
  findings: string;
  effectivenessRating: number;
}

export const RiskReviewPhase = () => {
  const { data: reviews, isLoading, error, isSubmitting, reviewRisk } = useRiskReview();
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [formData, setFormData] = useState({
    riskTitle: '',
    reviewType: 'Quarterly',
    findings: '',
    effectivenessRating: 75,
  });

  const reviewTypes = ['Quarterly', 'Annual', 'Exceptional', 'Post-Incident'];

  const handleAddReview = async () => {
    if (formData.riskTitle && formData.findings) {
      const success = await reviewRisk({
        risk_id: editingId || Date.now().toString(),
        review_type: formData.reviewType,
        review_date: new Date().toISOString().split('T')[0],
        findings: formData.findings,
        effectiveness_rating: formData.effectivenessRating,
      });

      if (success) {
        toast.success('Risk review saved successfully');
        setFormData({
          riskTitle: '',
          reviewType: 'Quarterly',
          findings: '',
          effectivenessRating: 75,
        });
        setShowForm(false);
        setEditingId(null);
      } else {
        toast.error('Failed to save risk review');
      }
    } else {
      toast.error('Please fill in all required fields');
    }
  };

  return (
    <div className="space-y-6">
      {/* Phase Info */}
      <Card>
        <div className="p-6">
          <div className="flex items-start gap-4">
            <Shield size={32} className="text-blue-500 flex-shrink-0 mt-1" />
            <div>
              <h3 className="text-xl font-bold mb-2">Phase 5: Risk Review</h3>
              <p className="text-zinc-400 mb-4">
                Conduct periodic and exceptional reviews to assess the effectiveness of risk management processes, identify improvements, and ensure continuous alignment with organizational objectives.
              </p>
              <div className="grid grid-cols-4 gap-4 text-sm">
                <div>
                  <p className="text-zinc-400">Total Reviews</p>
                  <p className="text-2xl font-bold">{isLoading ? '...' : reviews.length}</p>
                </div>
                <div>
                  <p className="text-zinc-400">Completed</p>
                  <p className="text-2xl font-bold text-green-400">{isLoading ? '...' : reviews.length}</p>
                </div>
                <div>
                  <p className="text-zinc-400">Avg. Effectiveness</p>
                  <p className="text-2xl font-bold">
                    {isLoading
                      ? '...'
                      : reviews.length > 0
                        ? (reviews.reduce((sum, r) => sum + r.effectivenessRating, 0) / reviews.length).toFixed(0)
                        : 0}
                    %
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">Status</p>
                  <p className="text-2xl font-bold text-blue-400">Active</p>
                </div>
              </div>
              {error && <p className="text-red-400 text-sm mt-3">{error}</p>}
            </div>
          </div>
        </div>
      </Card>

      {/* Review Form */}
      {showForm && (
        <Card>
          <div className="p-6">
            <h3 className="text-lg font-bold mb-4">{editingId ? 'Edit Risk Review' : 'Add Risk Review'}</h3>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium mb-2">Risk Title *</label>
                <input
                  type="text"
                  value={formData.riskTitle}
                  onChange={(e) => setFormData({ ...formData, riskTitle: e.target.value })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500"
                  placeholder="e.g., Data Breach Risk"
                  disabled={isSubmitting}
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Review Type *</label>
                <select
                  value={formData.reviewType}
                  onChange={(e) => setFormData({ ...formData, reviewType: e.target.value })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white focus:outline-none focus:border-blue-500"
                  disabled={isSubmitting}
                >
                  {reviewTypes.map((t) => (
                    <option key={t} value={t}>
                      {t}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Findings *</label>
                <textarea
                  value={formData.findings}
                  onChange={(e) => setFormData({ ...formData, findings: e.target.value })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500 h-20"
                  placeholder="Document review findings..."
                  disabled={isSubmitting}
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Effectiveness Rating (%)</label>
                <div className="flex items-center gap-4">
                  <input
                    type="range"
                    min="0"
                    max="100"
                    value={formData.effectivenessRating}
                    onChange={(e) => setFormData({ ...formData, effectivenessRating: parseInt(e.target.value) })}
                    className="flex-1"
                    disabled={isSubmitting}
                  />
                  <span className="text-lg font-bold w-12">{formData.effectivenessRating}%</span>
                </div>
              </div>

              <div className="flex gap-2 pt-4">
                <Button
                  onClick={handleAddReview}
                  disabled={isSubmitting}
                  className="bg-blue-600 hover:bg-blue-700 text-white flex items-center gap-2 disabled:opacity-50"
                >
                  {isSubmitting && <Loader2 size={16} className="animate-spin" />}
                  {editingId ? 'Update Review' : 'Add Review'}
                </Button>
                <Button
                  onClick={() => {
                    setShowForm(false);
                    setEditingId(null);
                    setFormData({
                      riskTitle: '',
                      reviewType: 'Quarterly',
                      findings: '',
                      effectivenessRating: 75,
                    });
                  }}
                  disabled={isSubmitting}
                  className="bg-zinc-700 hover:bg-zinc-600 text-white disabled:opacity-50"
                >
                  Cancel
                </Button>
              </div>
            </div>
          </div>
        </Card>
      )}

      {/* Reviews List */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-bold">Risk Reviews</h3>
          {!showForm && (
            <Button
              onClick={() => setShowForm(true)}
              className="bg-blue-600 hover:bg-blue-700 text-white flex items-center gap-2"
            >
              <Plus size={18} />
              Add Review
            </Button>
          )}
        </div>

        {isLoading && (
          <Card>
            <div className="p-12 text-center">
              <Loader2 size={48} className="mx-auto mb-4 text-zinc-500 animate-spin" />
              <p className="text-zinc-400">Loading risk reviews...</p>
            </div>
          </Card>
        )}

        {!isLoading && reviews.length === 0 && (
          <Card>
            <div className="p-12 text-center">
              <Shield size={48} className="mx-auto mb-4 text-zinc-500" />
              <p className="text-zinc-400">No reviews yet</p>
            </div>
          </Card>
        )}

        {!isLoading &&
          reviews.map((review, idx) => (
            <motion.div
              key={review.id}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.1 }}
            >
              <Card>
                <div className="p-6">
                  <div className="flex items-start justify-between">
                    <div className="flex-1">
                      <h3 className="text-lg font-bold mb-3">{review.riskTitle}</h3>
                      <div className="grid grid-cols-4 gap-4 mb-4">
                        <div>
                          <p className="text-xs text-zinc-500">Review Type</p>
                          <p className="text-sm font-medium">{review.reviewType}</p>
                        </div>
                        <div>
                          <p className="text-xs text-zinc-500">Review Date</p>
                          <p className="text-sm font-medium">{review.reviewDate}</p>
                        </div>
                        <div>
                          <p className="text-xs text-zinc-500">Effectiveness</p>
                          <p className="text-sm font-medium">{review.effectivenessRating}%</p>
                        </div>
                      </div>
                      <div>
                        <p className="text-xs text-zinc-500 mb-1">Findings</p>
                        <p className="text-sm">{review.findings}</p>
                      </div>
                    </div>

                    <div className="flex gap-2 ml-4">
                      <button
                        onClick={() => {
                          setFormData({
                            riskTitle: review.riskTitle,
                            reviewType: review.reviewType,
                            findings: review.findings,
                            effectivenessRating: review.effectivenessRating,
                          });
                          setEditingId(review.id);
                          setShowForm(true);
                        }}
                        className="text-zinc-400 hover:text-blue-500 transition-colors p-2"
                      >
                        <Edit2 size={20} />
                      </button>
                      <button
                        onClick={() => toast.info('Delete coming soon')}
                        className="text-zinc-400 hover:text-red-500 transition-colors p-2"
                      >
                        <Trash2 size={20} />
                      </button>
                    </div>
                  </div>
                </div>
              </Card>
            </motion.div>
          ))}
      </div>
    </div>
  );
};
