import { ResponsiveContainer, ScatterChart, Scatter, XAxis, YAxis, ZAxis, Tooltip, Cell } from 'recharts';

const data = [
  { x: , y: , z: , name: 'Server Patching', score:  }, // Low/Low
  { x: , y: , z: , name: 'Database Injection', score:  }, // High/High
  { x: , y: , z: , name: 'Phishing', score:  },
  { x: , y: , z: , name: 'DDOS', score:  },
  { x: , y: , z: , name: 'Insider Threat', score:  },
];

const COLORS = {
  Low: 'bf',
  Medium: 'feb',
  High: 'f',
  Critical: 'ef'
};

export const RiskHeatmap = () => {
  return (
    <div className="h-full w-full p-">
      <h className="text-sm font-medium text-zinc- mb- uppercase tracking-wider">Risk Heatmap</h>
      <ResponsiveContainer width="%" height="%">
        <ScatterChart margin={{ top: , right: , bottom: , left:  }}>
          <XAxis type="number" dataKey="x" name="Impact" unit="" domain={[, ]} tickCount={} stroke="b" />
          <YAxis type="number" dataKey="y" name="Probability" unit="" domain={[, ]} tickCount={} stroke="b" />
          <ZAxis type="number" dataKey="z" range={[, ]} />
          <Tooltip 
            cursor={{ strokeDasharray: ' ' }} 
            contentStyle={{ backgroundColor: 'b', borderColor: 'a', color: 'fff' }}
          />
          <Scatter name="Risks" data={data}>
            {data.map((entry, index) => {
               const color = entry.score >=  ? COLORS.Critical : entry.score >=  ? COLORS.Medium : COLORS.Low;
               return <Cell key={cell-${index}} fill={color} stroke="none" />;
            })}
          </Scatter>
        </ScatterChart>
      </ResponsiveContainer>
    </div>
  );
};