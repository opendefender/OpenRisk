import { PrioritizedMitigationsList } from '../features/mitigations/PrioritizedMitigationsList';

export const Recommendations = () => {
  return (
    <div className="p- h-full overflow-y-auto">
      <div className="max-w-xl mx-auto">
        <div className="mb-">
            <h className="text-xl font-bold text-white mb-">Intelligence & Recommendations</h>
            <p className="text-zinc-">
                Optimisez vos efforts de s√curit√ en traitant d'abord ce qui compte vraiment.
            </p>
        </div>
        
        
        <div className="bg-surface/ border border-border rounded-xl p-">
            <PrioritizedMitigationsList />
        </div>
      </div>
    </div>
  );
};