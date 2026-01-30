import { useEffect, useState, useMemo } from 'react';
import { api } from '../../../lib/api';
import { Loader } from 'lucide-react';

interface MatrixCellData {
    impact: number;
    probability: number;
    count: number;
}

// Fonction pour d√terminer la couleur du risque 
const getCellColor = (impact: number, probability: number, count: number) => {
    const score = impact  probability;
    if (count === ) return 'bg-zinc- border-zinc- hover:bg-zinc-';

    if (score >= ) return 'bg-red-/ border-red- ring-red-/';
    if (score >= ) return 'bg-orange-/ border-orange- ring-orange-/';
    if (score >= ) return 'bg-yellow-/ border-yellow- ring-yellow-/';
    return 'bg-blue-/ border-blue- ring-blue-/';
};

export const RiskMatrix = () => {
    const [data, setData] = useState<MatrixCellData[]>([]);
    const [isLoading, setIsLoading] = useState(true);

    useEffect(() => {
        api.get('/stats/risk-matrix')
            .then(res => setData(res.data))
            .catch(err => console.error("Failed to fetch matrix data:", err))
            .finally(() => setIsLoading(false));
    }, []);

    // Transformation des donn√es en un format de carte x
    const matrixMap = useMemo(() => {
        const map = new Map<string, number>();
        data.forEach(cell => {
            map.set(${cell.impact}-${cell.probability}, cell.count);
        });
        return map;
    }, [data]);

    if (isLoading) {
        return <div className="flex justify-center items-center h-full text-zinc-"><Loader className="animate-spin mr-" size={} /> Loading Matrix...</div>;
    }

    // Le Risk Matrix est g√n√ralement repr√sent√ avec la Probabilit√ sur l'axe Y et l'Impact sur l'axe X.
    const dimensions = [, , , , ]; // Pour l'axe Y (Probabilit√, de haut en bas)
    const columns = [, , , , ]; // Pour l'axe X (Impact)

    return (
        <div className="p-">
            <h className="text-lg font-bold text-white mb-">Risk Exposure Matrix (x)</h>
            
            <div className="flex">
                {/ Axe Y (Probabilit√) /}
                <div className="flex flex-col justify-end text-right pr- text-xs text-zinc- font-mono tracking-wider">
                    {dimensions.map(p => (
                        <div key={p} className="h- flex items-center justify-end">{p}</div>
                    ))}
                    <div className="h- text-white font-bold flex items-center justify-end">PROBA</div>
                </div>

                {/ La Grille (Cells) /}
                <div className="flex-">
                    <div className="grid grid-cols- gap-">
                        {dimensions.map(p => ( // Lignes (Probabilit√)
                            columns.map(i => { // Colonnes (Impact)
                                const key = ${i}-${p};
                                const count = matrixMap.get(key) || ;
                                const colorClass = getCellColor(i, p, count);
                                
                                return (
                                    <div 
                                        key={key} 
                                        className={h- flex items-center justify-center rounded-lg text-sm font-bold transition-all duration- cursor-pointer 
                                                    ${colorClass} ${count >  ? 'ring-' : ''}}
                                        title={Risks: ${count} | Score: ${i  p}}
                                    >
                                        {count >  ? (
                                            <span className="text-white text-md font-extrabold drop-shadow-md">{count}</span>
                                        ) : (
                                            <span className="text-zinc-/">-</span>
                                        )}
                                    </div>
                                );
                            })
                        ))}
                    </div>

                    {/ Axe X (Impact) /}
                    <div className="grid grid-cols- gap- mt-">
                        {columns.map(i => (
                            <div key={i} className="h- flex items-center justify-center text-xs text-zinc- font-mono tracking-wider">
                                {i}
                            </div>
                        ))}
                    </div>
                    <div className="text-center mt- text-white font-bold">IMPACT</div>
                </div>
            </div>
        </div>
    );
};