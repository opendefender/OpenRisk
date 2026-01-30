import { CircularProgressbarWithChildren, buildStyles } from 'react-circular-progressbar';
import 'react-circular-progressbar/dist/styles.css';
import { ShieldCheck } from 'lucide-react';

export const GlobalScore = ({ score }: { score: number }) => {
  return (
    <div className="h-full flex flex-col items-center justify-center p-">
      <div className="w- h- relative">
        <CircularProgressbarWithChildren
          value={score}
          styles={buildStyles({
            pathColor: score >  ? 'b' : score >  ? 'feb' : 'ef',
            trailColor: 'rgba(,,,.)',
            pathTransitionDuration: .,
          })}
        >
            <div className="flex flex-col items-center animate-fade-in">
                <ShieldCheck size={} className="text-zinc- mb-" />
                <span className="text-xl font-bold text-white">{score}</span>
                <span className="text-[px] uppercase text-zinc- tracking-widest">Sec. Score</span>
            </div>
        </CircularProgressbarWithChildren>
      </div>
      <p className="mt- text-center text-sm text-zinc-">
        Votre posture de scurit est <span className="text-emerald- font-medium">optimale</span>.
      </p>
    </div>
  );
};