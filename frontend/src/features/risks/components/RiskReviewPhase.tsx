import { Shield } from 'lucide-react';
import { motion } from 'framer-motion';
import { Card } from '../../../components/Card';

interface RiskReview {
  riskTitle: string;
  reviewType: string;
  status: string;
  findingsCount: number;
  nextReview: string;
}

export const RiskReviewPhase = () => {
  const reviews: RiskReview[] = [
    {
      riskTitle: 'Data Breach Risk',
      reviewType: 'Quarterly Review',
      status: 'Completed',
      findingsCount: 2,
      nextReview: '2024-05-01',
    },
    {
      riskTitle: 'Compliance Gap',
      reviewType: 'Annual Review',
      status: 'In Progress',
      findingsCount: 1,
      nextReview: '2024-05-15',
    },
  ];

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
                  <p className="text-2xl font-bold">{reviews.length}</p>
                </div>
                <div>
                  <p className="text-zinc-400">Completed</p>
                  <p className="text-2xl font-bold text-green-400">
                    {reviews.filter((r) => r.status === 'Completed').length}
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">In Progress</p>
                  <p className="text-2xl font-bold text-blue-400">
                    {reviews.filter((r) => r.status === 'In Progress').length}
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">Total Findings</p>
                  <p className="text-2xl font-bold">{reviews.reduce((sum, r) => sum + r.findingsCount, 0)}</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Card>

      {/* Reviews List */}
      <div className="space-y-3">
        <h3 className="text-lg font-bold">Risk Reviews</h3>

        {reviews.length === 0 ? (
          <Card>
            <div className="p-12 text-center">
              <Shield size={48} className="mx-auto mb-4 text-zinc-500" />
              <p className="text-zinc-400">No reviews scheduled</p>
            </div>
          </Card>
        ) : (
          reviews.map((review, idx) => (
            <motion.div
              key={`${review.riskTitle}-${idx}`}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.1 }}
            >
              <Card>
                <div className="p-6">
                  <div className="flex items-center justify-between mb-4">
                    <h3 className="text-lg font-bold">{review.riskTitle}</h3>
                    <span className={`text-xs px-2 py-1 rounded font-semibold ${
                      review.status === 'Completed' ? 'bg-green-500/20 text-green-400' : 'bg-blue-500/20 text-blue-400'
                    }`}>
                      {review.status}
                    </span>
                  </div>

                  <div className="grid grid-cols-4 gap-4">
                    <div>
                      <p className="text-xs text-zinc-500">Review Type</p>
                      <p className="text-sm font-medium">{review.reviewType}</p>
                    </div>
                    <div>
                      <p className="text-xs text-zinc-500">Findings</p>
                      <p className="text-sm font-medium">{review.findingsCount}</p>
                    </div>
                    <div>
                      <p className="text-xs text-zinc-500">Next Review</p>
                      <p className="text-sm font-medium">{review.nextReview}</p>
                    </div>
                    <div>
                      <p className="text-xs text-zinc-500">Status</p>
                      <p className="text-sm font-medium">{review.status}</p>
                    </div>
                  </div>
                </div>
              </Card>
            </motion.div>
          ))
        )}
      </div>
    </div>
  );
};
