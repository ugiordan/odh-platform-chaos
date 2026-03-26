interface ProgressBarProps {
  resilient: number;
  degraded: number;
  failed: number;
}

export function ProgressBar({ resilient, degraded, failed }: ProgressBarProps) {
  const total = resilient + degraded + failed;

  if (total === 0) {
    return <div className="progress-bar"></div>;
  }

  const segments = [
    { value: resilient, color: 'green' },
    { value: degraded, color: 'yellow' },
    { value: failed, color: 'red' },
  ].filter(seg => seg.value > 0);

  return (
    <div className="progress-bar">
      {segments.map((seg, idx) => (
        <div
          key={idx}
          className={`progress-segment ${seg.color}`}
          style={{ width: `${(seg.value / total) * 100}%` }}
        />
      ))}
    </div>
  );
}
